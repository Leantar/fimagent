package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func FromYamlFile(path string, dest interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if !stat.Mode().IsRegular() {
		return errors.New("config: config file is not regular")
	}

	if err := yaml.NewDecoder(f).Decode(dest); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return nil
}
