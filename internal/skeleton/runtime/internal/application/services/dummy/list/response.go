package list

// Item é um Dummy na saída do use case List.
type Item struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Response agrega a saída do use case List.
type Response struct {
	Items []Item `json:"items"`
}
