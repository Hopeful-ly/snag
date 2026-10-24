package nativehost

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func setRegistry(key, manifest string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("", manifest)
}

func deleteRegistry(key string) error {
	err := registry.DeleteKey(registry.CURRENT_USER, key)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}
