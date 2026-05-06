package repos

import (
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type OrderRepo interface {
	Create()
	Fetch()
	FetchAll()
	Delete()
}

type pgOrderRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewPGOrderRepo(db *sqlx.DB, logger *zap.Logger) OrderRepo {
	return pgOrderRepo{db: db, logger: logger}
}

func (p pgOrderRepo) Create() {
	panic("unimplemented")
}

func (p pgOrderRepo) Delete() {
	panic("unimplemented")
}

func (p pgOrderRepo) Fetch() {
	panic("unimplemented")
}

func (p pgOrderRepo) FetchAll() {
	panic("unimplemented")
}
