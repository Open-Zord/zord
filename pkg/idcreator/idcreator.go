package idcreator

import "github.com/oklog/ulid/v2"

// RegistryKey is the key under which the id creator is registered in the registry.
const RegistryKey = "idCreator"

type IDCreator struct {
}

func NewIDCreator() *IDCreator {
	return &IDCreator{}
}

func (*IDCreator) Create() string {
	return ulid.Make().String()
}
