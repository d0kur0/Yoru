package core

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// RouteSetsFile is a portable package, separate from a full configuration backup.
type RouteSetsFile struct {
	Format  string     `json:"format"`
	Version int        `json:"version"`
	Sets    []RouteSet `json:"sets"`
}

const routeSetsFormat = "yoru-route-sets"

func DecodeRouteSets(data []byte) ([]RouteSet, error) {
	if len(data) > 1<<20 {
		return nil, errors.New("Файл наборов больше 1 МБ")
	}
	var file RouteSetsFile
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("Некорректный JSON наборов: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("Файл должен содержать один JSON-документ")
	}
	if file.Format != routeSetsFormat || file.Version != 1 {
		return nil, errors.New("Нужен файл наборов Yoru версии 1, не резервная копия настроек")
	}
	if len(file.Sets) == 0 || len(file.Sets) > 100 {
		return nil, errors.New("В файле должно быть от 1 до 100 наборов")
	}
	for i := range file.Sets {
		// Imported identifiers never replace a local set with the same identifier.
		token := make([]byte, 16)
		if _, err := rand.Read(token); err != nil {
			return nil, err
		}
		file.Sets[i].ID = hex.EncodeToString(token)
		file.Sets[i].Name = strings.TrimSpace(file.Sets[i].Name)
	}
	config := DefaultConfig()
	config.Sets = file.Sets
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("Наборы не импортированы: %w", err)
	}
	return file.Sets, nil
}

func ReadRouteSets(path string) ([]RouteSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := readLimit(file, 1<<20)
	if err != nil {
		return nil, err
	}
	return DecodeRouteSets(data)
}

func EncodeRouteSets(sets []RouteSet) ([]byte, error) {
	if len(sets) == 0 {
		return nil, errors.New("Нет наборов для экспорта")
	}
	portable := append([]RouteSet(nil), sets...)
	for i := range portable {
		portable[i].ID = ""
	}
	data, err := json.MarshalIndent(RouteSetsFile{routeSetsFormat, 1, portable}, "", "  ")
	if err != nil {
		return nil, err
	}
	if _, err = DecodeRouteSets(data); err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func WriteRouteSets(path string, sets []RouteSet) error {
	data, err := EncodeRouteSets(sets)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0600)
}
