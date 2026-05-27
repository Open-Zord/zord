package dummy

import (
	domain "github.com/Open-Zord/zord/internal/application/domain/dummy"
)

type Response struct {
	CurrentPage int
	TotalPages  int64
	Data        *[]domain.Dummy
}
