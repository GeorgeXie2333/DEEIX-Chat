package repository

// OpenWebUIUserRow is a normalized user row read from an OpenWebUI database.
type OpenWebUIUserRow struct {
	PublicID    string
	Username    string
	DisplayName string
	Email       string
	Balance     float64
}
