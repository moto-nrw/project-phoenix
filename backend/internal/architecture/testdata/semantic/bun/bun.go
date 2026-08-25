package bun

type BaseModel struct{}

type DB struct{}

type SelectQuery struct{}
type InsertQuery struct{}
type UpdateQuery struct{}
type DeleteQuery struct{}

func (*DB) NewSelect() *SelectQuery { return &SelectQuery{} }
func (*DB) NewInsert() *InsertQuery { return &InsertQuery{} }
func (*DB) NewUpdate() *UpdateQuery { return &UpdateQuery{} }
func (*DB) NewDelete() *DeleteQuery { return &DeleteQuery{} }
func (*DB) Exec(string, ...any)     {}

func (q *SelectQuery) TableExpr(string, ...any) *SelectQuery { return q }
func (q *SelectQuery) Join(string, ...any) *SelectQuery      { return q }
func (q *SelectQuery) ColumnExpr(string, ...any) *SelectQuery {
	return q
}
func (q *SelectQuery) For(string, ...any) *SelectQuery       { return q }
func (q *InsertQuery) TableExpr(string, ...any) *InsertQuery { return q }
func (q *InsertQuery) On(string, ...any) *InsertQuery        { return q }
func (q *UpdateQuery) TableExpr(string, ...any) *UpdateQuery { return q }
func (q *DeleteQuery) TableExpr(string, ...any) *DeleteQuery { return q }

func (q *InsertQuery) Model(any) *InsertQuery { return q }
