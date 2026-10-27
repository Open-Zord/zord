package edit

import "errors"

// Data agrega os campos de entrada validáveis do use case Edit.
type Data struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Request encapsula Data para o Execute do Service.
type Request struct {
	Data *Data
}

// NewRequest constrói o Request.
func NewRequest(data *Data) *Request {
	return &Request{Data: data}
}

// Validate garante que o ID foi informado.
func (r *Request) Validate() error {
	if r.Data == nil || r.Data.ID == "" {
		return errors.New("id é obrigatório")
	}
	return nil
}
