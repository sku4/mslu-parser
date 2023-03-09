package repository

//go:generate mockgen -source=repository.go -destination=mocks/repository.go

type Repository struct {
}

// NewRepository created Repository struct
func NewRepository() *Repository {
	return &Repository{}
}
