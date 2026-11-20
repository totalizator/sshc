package ssh

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kevinburke/ssh_config"
)

// BackupsKept is how many earlier versions of the config sshc keeps: the
// newest at <path>.sshc.bak, older ones at <path>.sshc.bak.1 … .bak.4.
//
// Only writes that change the config's content rotate them (add, edit,
// delete, clean, import, restore). Stamping used= on connect and toggling a
// favourite don't, or a few connects would push the last real edit out.
const BackupsKept = 5

// backupName is the path of backup n: 0 is the newest.
func backupName(path string, n int) string {
	if n == 0 {
		return path + backupSuffix
	}
	return path + backupSuffix + "." + strconv.Itoa(n)
}

// backupFile saves data (the config as it was before a write) as the newest
// backup, shifting the older ones down and dropping the oldest.
func backupFile(path string, data []byte) error {
	for n := BackupsKept - 1; n > 0; n-- {
		err := os.Rename(backupName(path, n-1), backupName(path, n))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	if err := os.WriteFile(backupName(path, 0), data, 0o600); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	return nil
}

// Backup is one saved earlier version of a config.
type Backup struct {
	Path    string
	ModTime time.Time // when it was replaced, i.e. when the write after it happened
	Size    int64
	Hosts   int // concrete host entries in it; -1 when it doesn't parse
}

// Backups lists the backups of the config at path, newest first.
func Backups(path string) []Backup {
	var out []Backup
	for n := 0; n < BackupsKept; n++ {
		name := backupName(path, n)
		fi, err := os.Stat(name)
		if err != nil {
			continue
		}
		b := Backup{Path: name, ModTime: fi.ModTime(), Size: fi.Size(), Hosts: -1}
		if hosts, err := loadFile(name, false); err == nil {
			b.Hosts = len(hosts)
		}
		out = append(out, b)
	}
	return out // by slot, which rotation keeps newest first
}

// Replace writes data as the whole config at path, after checking it parses,
// and backs up the current content first. It is how an import or a restore
// lands, so both can be undone the same way.
func Replace(path string, data []byte) error {
	if _, err := ssh_config.DecodeBytes(data); err != nil {
		return fmt.Errorf("not a valid ssh config: %w", err)
	}
	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if original != nil {
		if err := backupFile(path, original); err != nil {
			return err
		}
	}
	return writeFile(path, []byte(stripControlBytes(string(data))))
}

// Restore puts backup back as the config at path. backup must be one of the
// paths Backups lists. The current config becomes the newest backup, so a
// restore can itself be undone.
func Restore(path, backup string) error {
	known := false
	for n := 0; n < BackupsKept; n++ {
		if backup == backupName(path, n) {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("%s is not a backup of %s", backup, path)
	}
	data, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	return Replace(path, data)
}
