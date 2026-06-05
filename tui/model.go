package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	fz "github.com/totalizator/sshc/fuzzy"
	"github.com/totalizator/sshc/ssh"
)

type state int

const (
	stateList state = iota
	stateForm
	stateConfirm
	statePromptUser
)

// Settings holds the user-facing UI configuration. The locked defaults ship as
// framed · amber · comfortable (handoff §1); everything is overridable.
type Settings struct {
	Theme          string // amber | teal | green | magenta (or a #hex)
	Variant        Variant
	Density        Density
	DetailDefault  bool
	PinFavorites   bool
	VerboseDefault bool
	EnvColors      map[string]string
}

// DefaultSettings returns the shipped defaults.
func DefaultSettings() Settings {
	return Settings{
		Theme:          "amber",
		Variant:        VariantFramed,
		Density:        DensityComfortable,
		DetailDefault:  false,
		PinFavorites:   true,
		VerboseDefault: false,
	}
}

// Options configures a TUI run.
type Options struct {
	ConfigPaths []string // first entry is the writable primary config
	Hosts       []ssh.Host
	Filter      string
	Template    string
	SortByName  bool
	ShowProxy   bool
	Version     string
	Settings    Settings
}

func (o Options) primaryConfig() string {
	if len(o.ConfigPaths) > 0 {
		return o.ConfigPaths[0]
	}
	return ""
}

type toast struct {
	text string
	kind string // ok | go | warn | mut
}

// Model is the root bubbletea model and state machine.
type Model struct {
	opts     Options
	settings Settings
	styles   Styles
	state    state

	search textinput.Model
	hosts  []ssh.Host

	// computed view
	rows    []viewRow
	visible []ssh.Host
	sel     int
	query   string

	showDetail bool
	showHelp   bool
	verbose    bool

	showSettings  bool
	settingsFocus int

	form        formModel
	confirmHost ssh.Host

	// username prompt shown at connect time when a host has no saved User.
	userInput   textinput.Model
	promptHost  *ssh.Host
	connectUser string // session-only username collected from the prompt

	toast    toast
	toastSeq int

	width, height int

	connect *ssh.Host
}

type clearToastMsg int

// Run starts the TUI and, on a connect request, stamps last-used and execs ssh.
func Run(opts Options) error {
	m := newModel(opts)
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	fm, ok := final.(Model)
	if !ok || fm.connect == nil {
		return nil
	}
	// Best-effort recency bookkeeping; never blocks the connection.
	_ = ssh.TouchLastUsed(opts.primaryConfig(), fm.connect.Alias, time.Now())
	return connect(*fm.connect, opts.Template, fm.connectUser)
}

func newModel(opts Options) Model {
	set := opts.Settings
	if set.Theme == "" {
		set = DefaultSettings()
	}

	search := textinput.New()
	search.Placeholder = "type to filter — matches any field"
	search.Focus()

	m := Model{
		opts:       opts,
		settings:   set,
		hosts:      append([]ssh.Host(nil), opts.Hosts...),
		search:     search,
		verbose:    set.VerboseDefault,
		showDetail: set.DetailDefault,
	}
	m.rebuildStyles()
	if strings.TrimSpace(opts.Filter) != "" {
		m.search.SetValue(opts.Filter)
	}
	m.applyFilter()
	return m
}

// rebuildStyles recomputes the theme from m.settings and re-applies it to the
// search input. Called on startup and whenever the settings overlay changes a
// value, so the look updates live.
func (m *Model) rebuildStyles() {
	m.styles = NewStyles(m.settings.Theme, m.settings.Variant, m.settings.Density, m.settings.EnvColors)
	m.search.Prompt = m.styles.SearchGlyph.Render("/") + " "
	m.search.PlaceholderStyle = m.styles.SearchPH
	m.search.TextStyle = m.styles.SearchText
	m.search.Cursor.Style = lipgloss.NewStyle().Foreground(color(m.styles.Accent))
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case clearToastMsg:
		if int(msg) == m.toastSeq {
			m.toast = toast{}
		}
		return m, nil
	}

	switch m.state {
	case stateForm:
		return m.updateForm(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	case statePromptUser:
		return m.updateUserPrompt(msg)
	default:
		return m.updateList(msg)
	}
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, isKey := msg.(tea.KeyMsg)
	if !isKey {
		return m, nil
	}

	if m.showHelp {
		switch km.String() {
		case "?", "esc", "f1", "ctrl+c":
			m.showHelp = false
		}
		return m, nil
	}

	if m.showSettings {
		return m.updateSettings(km)
	}

	switch km.String() {
	case "ctrl+t":
		m.showSettings = true
		m.settingsFocus = 0
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "?":
		if m.search.Value() == "" {
			m.showHelp = true
			return m, nil
		}
	case "f1":
		m.showHelp = true
		return m, nil
	case "esc":
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.applyFilter()
			return m, nil
		}
		return m, tea.Quit
	case "enter":
		if h, ok := m.selectedHost(); ok {
			if strings.TrimSpace(h.User) == "" {
				return m.startUserPrompt(h)
			}
			m.connect = &h
			return m, tea.Quit
		}
		return m, nil
	case "up", "ctrl+k":
		m.moveSel(-1)
		return m, nil
	case "down", "ctrl+j":
		m.moveSel(1)
		return m, nil
	case "pgup":
		m.moveSel(-8)
		return m, nil
	case "pgdown":
		m.moveSel(8)
		return m, nil
	case "home":
		m.sel = 0
		return m, nil
	case "end":
		m.sel = len(m.visible) - 1
		return m, nil
	case "tab":
		m.showDetail = !m.showDetail
		return m, nil
	case "ctrl+v":
		m.verbose = !m.verbose
		return m, nil
	case "ctrl+n":
		return m.openForm(newFormModel())
	case "ctrl+e":
		return m.startEdit()
	case "ctrl+l":
		return m.startClone()
	case "ctrl+f":
		return m.toggleFav()
	case "ctrl+d":
		return m.startDelete()
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *Model) moveSel(d int) {
	if len(m.visible) == 0 {
		return
	}
	m.sel += d
	if m.sel < 0 {
		m.sel = 0
	}
	if m.sel > len(m.visible)-1 {
		m.sel = len(m.visible) - 1
	}
}

func (m Model) selectedHost() (ssh.Host, bool) {
	if m.sel >= 0 && m.sel < len(m.visible) {
		return m.visible[m.sel], true
	}
	return ssh.Host{}, false
}

// stripControl removes control runes (e.g. a NUL from a Ctrl+Space keystroke)
// from a string, so they never enter a field value, a derived chip colour, or
// the persisted config.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// currentUsername returns the local login name — what ssh connects as when no
// User is set — with any Windows DOMAIN\ prefix stripped. Empty if unreadable.
func currentUsername() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return ""
	}
	name := u.Username
	if i := strings.LastIndexAny(name, `\/`); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// startUserPrompt opens the connect-time username prompt for a host that has no
// saved User, mirroring ssh's own lack of a default rather than forcing one.
func (m Model) startUserPrompt(h ssh.Host) (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "leave blank for ssh default"
	// No prompt glyph: an "@" here read as if it prefixed the username. The
	// user@host shape is shown live in the preview line instead (userPromptBox).
	ti.Prompt = ""
	ti.PlaceholderStyle = m.styles.SearchPH
	ti.TextStyle = m.styles.SearchText
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(color(m.styles.Accent))
	ti.Focus()
	m.userInput = ti
	m.promptHost = &h
	m.state = statePromptUser
	return m, textinput.Blink
}

// updateUserPrompt handles the username prompt: Enter connects (an empty value
// falls back to ssh's default), Esc cancels back to the list.
func (m Model) updateUserPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.state = stateList
			m.promptHost = nil
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			h := *m.promptHost
			m.connectUser = strings.TrimSpace(m.userInput.Value())
			m.connect = &h
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.userInput, cmd = m.userInput.Update(msg)
	return m, cmd
}

// applyFilter recomputes rows/visible from the current search query, grouping
// and sorting per the design (handoff §6).
func (m *Model) applyFilter() {
	q := strings.TrimSpace(m.search.Value())
	m.query = q
	m.rows = m.rows[:0]
	m.visible = m.visible[:0]

	addHost := func(h ssh.Host) {
		m.rows = append(m.rows, viewRow{host: h, selIdx: len(m.visible)})
		m.visible = append(m.visible, h)
	}
	addHeader := func(label, glyph string) {
		m.rows = append(m.rows, viewRow{header: true, label: label, glyph: glyph, selIdx: -1})
	}

	switch {
	case q != "":
		for _, h := range m.searchHosts(q) {
			addHost(h)
		}
	case m.opts.SortByName:
		for _, h := range fz.SortByName(m.hosts) {
			addHost(h)
		}
	case m.settings.PinFavorites:
		sorted := byRecency(m.hosts)
		var favs, rest []ssh.Host
		for _, h := range sorted {
			if h.Fav {
				favs = append(favs, h)
			} else {
				rest = append(rest, h)
			}
		}
		if len(favs) > 0 {
			addHeader("pinned", "★")
			for _, h := range favs {
				addHost(h)
			}
		}
		addHeader("all hosts", "")
		for _, h := range rest {
			addHost(h)
		}
	default:
		for _, h := range byRecency(m.hosts) {
			addHost(h)
		}
	}

	if m.sel > len(m.visible)-1 {
		m.sel = len(m.visible) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

// searchHosts ranks hosts against q by score then recency (handoff §6).
func (m Model) searchHosts(q string) []ssh.Host {
	type scored struct {
		h ssh.Host
		s int
	}
	var matched []scored
	for _, h := range m.hosts {
		if s, ok := scoreHost(q, h); ok {
			matched = append(matched, scored{h, s})
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].s != matched[j].s {
			return matched[i].s < matched[j].s
		}
		return recencyLess(matched[i].h, matched[j].h)
	})
	out := make([]ssh.Host, len(matched))
	for i, m := range matched {
		out[i] = m.h
	}
	return out
}

// byRecency returns hosts most-recently-used first; never-used sort last.
func byRecency(hosts []ssh.Host) []ssh.Host {
	out := append([]ssh.Host(nil), hosts...)
	sort.SliceStable(out, func(i, j int) bool {
		return recencyLess(out[i], out[j])
	})
	return out
}

func recencyLess(a, b ssh.Host) bool {
	az, bz := a.LastUsed.IsZero(), b.LastUsed.IsZero()
	if az != bz {
		return !az // a used, b never → a first
	}
	if az && bz {
		return false // both never used → keep stable order
	}
	return a.LastUsed.After(b.LastUsed)
}

// --- form / confirm / fav flows ---

func (m Model) startEdit() (tea.Model, tea.Cmd) {
	h, ok := m.selectedHost()
	if !ok {
		return m, nil
	}
	if !h.Editable {
		return m.flash("read-only host cannot be edited", "warn")
	}
	return m.openForm(formFromHost(h, "edit "+h.Alias, h.Alias, true))
}

func (m Model) startClone() (tea.Model, tea.Cmd) {
	h, ok := m.selectedHost()
	if !ok {
		return m, nil
	}
	name := m.uniqueCopyName(h.Alias)
	return m.openForm(formFromHost(h, "clone → "+name, name, false))
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	h, ok := m.selectedHost()
	if !ok {
		return m, nil
	}
	if !h.Editable {
		return m.flash("read-only host cannot be deleted", "warn")
	}
	m.confirmHost = h
	m.state = stateConfirm
	return m, nil
}

func (m Model) toggleFav() (tea.Model, tea.Cmd) {
	h, ok := m.selectedHost()
	if !ok {
		return m, nil
	}
	if !h.Editable {
		return m.flash("read-only host cannot be pinned", "warn")
	}
	if err := ssh.SetFav(m.opts.primaryConfig(), h.Alias, !h.Fav); err != nil {
		return m.flash("pin failed: "+err.Error(), "warn")
	}
	verb := "pinned "
	if h.Fav {
		verb = "unpinned "
	}
	return m.reload(verb+h.Alias, "ok", h.Alias)
}

func (m Model) openForm(f formModel) (tea.Model, tea.Cmd) {
	m.form = f
	m.state = stateForm
	m.toast = toast{}
	return m, nil
}

func (m Model) uniqueCopyName(base string) string {
	existing := make(map[string]bool, len(m.hosts))
	for _, h := range m.hosts {
		existing[h.Alias] = true
	}
	if cand := base + "-copy"; !existing[cand] {
		return cand
	}
	for i := 2; ; i++ {
		if cand := fmt.Sprintf("%s-copy%d", base, i); !existing[cand] {
			return cand
		}
	}
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc":
		m.state = stateList
		return m.flash("cancelled", "mut")
	case "enter", "ctrl+s":
		return m.submitForm()
	case "down", "tab", "ctrl+j":
		m.form.moveFocus(1)
	case "up", "shift+tab", "ctrl+k":
		m.form.moveFocus(-1)
	case "backspace":
		m.form.backspace()
	default:
		switch km.Type {
		case tea.KeyRunes:
			if !km.Alt {
				m.form.insert(string(km.Runes))
			}
		case tea.KeySpace:
			m.form.insert(" ")
		}
	}
	return m, nil
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	if msg := m.form.validate(); msg != "" {
		m.form.err = msg
		return m, nil
	}
	newHost := m.form.toHost(m.opts.primaryConfig())

	// Inline uniqueness check for a friendly in-form error.
	for _, h := range m.hosts {
		if h.Alias == newHost.Alias && newHost.Alias != m.form.editAlias {
			m.form.err = fmt.Sprintf("name %q already exists", newHost.Alias)
			return m, nil
		}
	}

	var err error
	if m.form.editAlias == "" {
		err = ssh.Add(m.opts.primaryConfig(), newHost)
	} else {
		err = ssh.Update(m.opts.primaryConfig(), m.form.editAlias, newHost)
	}
	if err != nil {
		m.form.err = err.Error()
		return m, nil
	}
	m.state = stateList
	return m.reload("saved "+newHost.Alias, "ok", newHost.Alias)
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "y", "Y", "enter":
		err := ssh.Delete(m.opts.primaryConfig(), m.confirmHost.Alias)
		m.state = stateList
		if err != nil {
			return m.flash("delete failed: "+err.Error(), "warn")
		}
		return m.reload("deleted "+m.confirmHost.Alias, "warn", "")
	case "n", "N", "esc":
		m.state = stateList
	}
	return m, nil
}

// reload re-reads all config files and rebuilds the list, then flashes a toast.
// When focusAlias is non-empty, the cursor follows that host across the rebuild
// (e.g. so a just-pinned host stays selected as it floats into ★ PINNED); pass
// "" to leave the cursor at its clamped position (e.g. after a delete, where the
// host is gone).
func (m Model) reload(text, kind, focusAlias string) (tea.Model, tea.Cmd) {
	hosts, err := ssh.Load(m.opts.ConfigPaths)
	if err != nil {
		return m.flash("reload failed: "+err.Error(), "warn")
	}
	m.hosts = hosts
	m.applyFilter()
	if focusAlias != "" {
		m.selectAlias(focusAlias)
	}
	return m.flash(text, kind)
}

// selectAlias moves the cursor to the visible host with the given alias, if it
// is present. It lets the selection follow a host across a reload that re-groups
// the list; a no-op (leaving applyFilter's clamped position) when not found.
func (m *Model) selectAlias(alias string) {
	for i, h := range m.visible {
		if h.Alias == alias {
			m.sel = i
			return
		}
	}
}

func (m Model) flash(text, kind string) (Model, tea.Cmd) {
	m.toast = toast{text: text, kind: kind}
	m.toastSeq++
	seq := m.toastSeq
	return m, tea.Tick(2600*time.Millisecond, func(time.Time) tea.Msg { return clearToastMsg(seq) })
}

// connect builds the exec command from the template and runs it. sessionUser,
// when non-empty, is a username collected from the connect-time prompt for a
// host with no saved User; it is applied for this invocation only.
func connect(h ssh.Host, tmpl, sessionUser string) error {
	args, err := connectArgs(h, tmpl, sessionUser)
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// connectArgs renders the connect template into a command + args. It is split
// out from connect so the argument handling (notably -l injection) is testable
// without execing anything.
func connectArgs(h ssh.Host, tmpl, sessionUser string) ([]string, error) {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = `ssh "{{{name}}}"`
	}
	user := h.User
	if sessionUser != "" {
		user = sessionUser
	}
	line := strings.NewReplacer(
		"{{{name}}}", h.Alias,
		"{{{hostname}}}", h.HostName,
		"{{{user}}}", user,
		"{{{port}}}", h.DisplayPort(),
	).Replace(tmpl)

	args := splitArgs(line)
	if len(args) == 0 {
		return nil, fmt.Errorf("empty connect command")
	}
	// When the prompt supplied a username but the template carries no
	// {{{user}}} placeholder (e.g. the default `ssh "{{{name}}}"`), hand it to
	// ssh explicitly with -l so the prompt actually takes effect.
	if sessionUser != "" && !strings.Contains(tmpl, "{{{user}}}") && looksLikeSSH(args[0]) {
		args = append([]string{args[0], "-l", sessionUser}, args[1:]...)
	}
	return args, nil
}

// looksLikeSSH reports whether cmd is the ssh client, so a prompted username
// can be safely injected with -l. Guards against breaking exotic custom
// templates (e.g. mosh) that don't take ssh's flags.
func looksLikeSSH(cmd string) bool {
	base := strings.ToLower(filepath.Base(cmd))
	base = strings.TrimSuffix(base, ".exe")
	return base == "ssh"
}

// splitArgs splits a command line into arguments, honoring single and double
// quotes. It is intentionally minimal — enough for the supported templates.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	var quote rune

	flush := func() {
		if cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return args
}
