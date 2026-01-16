package cache

// Cache defines session cache behavior.
type Cache interface {
	SetSession(token string, userID int64) error
}
