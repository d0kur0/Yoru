package core

import (
	"encoding/json"
	"errors"
	"os"
)

type Backup struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Config  Config `json:"config"`
}

func (m *Manager) Export(path string) error {
	b, e := json.MarshalIndent(Backup{"mihomo-desktop", 1, m.Config()}, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(path, b, 0600)
}
func ReadBackup(path string) (Config, error) {
	f, e := os.Open(path)
	if e != nil {
		return Config{}, e
	}
	defer f.Close()
	b, e := readLimit(f, 2<<20)
	if e != nil {
		return Config{}, e
	}
	var backup Backup
	if json.Unmarshal(b, &backup) != nil || backup.Version != 1 || (backup.Format != "mihomo-desktop" && backup.Format != "mihomo-desktop-prototype") {
		return Config{}, errors.New("Нужен файл настроек Mihomo Desktop версии 1")
	}
	backup.Config.normalize()
	return backup.Config, backup.Config.Validate()
}
