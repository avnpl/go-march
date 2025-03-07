package models

type Post struct {
	PostID    string `db:"post_id"`
	Title     string `db:"title"`
	Body      string `db:"body"`
	AuthorID  string `db:"author_id"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

type Transaction struct {
	TxnID        string `db:"txn_id"`
	Amount       int    `db:"amount"`
	SourceUserID string `db:"source_user_id"`
	TargetUserID string `db:"target_user_id"`
	CreatedAt    string `db:"created_at"`
	UpdatedAt    string `db:"updated_at"`
}

type User struct {
	UserID    string `db:"user_id"`
	Username  string `db:"username"`
	Bio       string `db:"bio"`
	EmailID   string `db:"email_id"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}
