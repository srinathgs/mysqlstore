package mysqlstore

import (
	"time"
)

var defaultInterval = time.Minute * 5

// Cleanup runs a background goroutine every interval that deletes expired
// sessions from the database.
//
// The design is based on https://github.com/yosssi/boltstore
func (m *MySQLStore) Cleanup(interval time.Duration) (chan<- struct{}, <-chan struct{}, <-chan error) {
	if interval <= 0 {
		interval = defaultInterval
	}

	quit, done, errChan := make(chan struct{}), make(chan struct{}), make(chan error)
	go m.cleanup(interval, quit, done, errChan)
	return quit, done, errChan
}

// StopCleanup stops the background cleanup from running.
func (m *MySQLStore) StopCleanup(quit chan<- struct{}, done <-chan struct{}) {
	quit <- struct{}{}
	<-done
}

// cleanup deletes expired sessions at set intervals.
func (m *MySQLStore) cleanup(interval time.Duration, quit <-chan struct{}, done chan<- struct{}, errChan chan<- error) {
	ticker := time.NewTicker(interval)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-quit:
			// Handle the quit signal.
			done <- struct{}{}
			return
		case <-ticker.C:
			// Delete expired sessions on each tick.
			go func() {
				err := m.deleteExpired()
				if err != nil {
					// it might be useful to return the errors in a channel to the caller.
					// it's the responsibility of the caller to range on this channel,
					errChan <- err
				}
			}()

		}
	}
}

// deleteExpired deletes expired sessions from the database.
func (m *MySQLStore) deleteExpired() error {
	var deleteStmt = "DELETE FROM " + m.table + " WHERE expires_on < NOW()"
	_, err := m.db.Exec(deleteStmt)
	return err
}
