package application

type Repository struct{ db DB }

func New(db DB) *Repository { return &Repository{db: db} }
