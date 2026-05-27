package idCreator

import "github.com/oklog/ulid/v2"

// RegistryKey is the key under which the id creator is registered in the registry.
const RegistryKey = "idCreator"

type IdCreator struct {
}

func NewIdCreator() *IdCreator {
	return &IdCreator{}
}

func (*IdCreator) Create() string {
	return ulid.Make().String()
}
