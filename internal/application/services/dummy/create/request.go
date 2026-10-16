package create

// Data agrega os campos de entrada do use case Create.
type Data struct {
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

// Validate valida Data. Sem validator configurado, retorna nil.
func (r *Request) Validate() error {
	return nil
}
