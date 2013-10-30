/*The MIT License (MIT)

Copyright (c) 2013 Srinath G.S.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package mysqlstore

import (
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"net/http"
	"time"
)

type MySQLStore struct {
	mysqlAuth string
	Codecs    []securecookie.Codec
	Options   *sessions.Options
	table     string
}

type sessionRow struct {
	id         string
	data       string
	createdOn  time.Time
	modifiedOn time.Time
	expiresOn  time.Time
}

func init() {
	gob.Register(time.Time{})
}

func NewMySQLStore(endpoint string, tableName string, path string, maxAge int, keyPairs ...[]byte) (*MySQLStore, error) {
	db, err := sql.Open("mysql", endpoint)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	cTableQ := "CREATE TABLE IF NOT EXISTS " + tableName + " (id INT NOT NULL AUTO_INCREMENT, session_data LONGBLOB, created_on TIMESTAMP DEFAULT 0, modified_on TIMESTAMP NOT NULL ON UPDATE CURRENT_TIMESTAMP, expires_on TIMESTAMP DEFAULT 0, PRIMARY KEY(`id`)) ENGINE=InnoDB"
	if _, err = db.Query(cTableQ); err != nil {
		return nil, err
	}
	return &MySQLStore{
		mysqlAuth: endpoint,
		Codecs:    securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   path,
			MaxAge: maxAge,
		},
		table: tableName,
	}, nil
}

func (m *MySQLStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(m, name)
}

func (m *MySQLStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &(*m.Options)
	session.IsNew = true
	var err error
	if cook, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, cook.Value, &session.ID, m.Codecs...)
		if err == nil {
			err = m.load(session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

func (m *MySQLStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	var err error
	if session.ID == "" {
		if err = m.insert(session); err != nil {
			return err
		}
	} else if err = m.save(session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, m.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (m *MySQLStore) insert(session *sessions.Session) error {
	var createdOn time.Time
	var modifiedOn time.Time
	var expiresOn time.Time
	crOn := session.Values["created_on"]
	if crOn == nil {
		createdOn = time.Now()
	} else {
		createdOn = crOn.(time.Time)
	}
	modifiedOn = createdOn
	exOn := session.Values["expires_on"]
	if exOn == nil {
		expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
	} else {
		expiresOn = exOn.(time.Time)
	}
	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")
	db, err := sql.Open("mysql", m.mysqlAuth)
	if err != nil {
		return err
	}
	defer db.Close()

	insQ := "INSERT INTO " + m.table + "(id, session_data, created_on, modified_on, expires_on) VALUES (NULL, ?, ?, ?, ?)"
	stmt, stmtErr := db.Prepare(insQ)
	if stmtErr != nil {
		return stmtErr
	}
	defer stmt.Close()
	encoded, encErr := securecookie.EncodeMulti(session.Name(), session.Values, m.Codecs...)
	if encErr != nil {
		return encErr
	}
	res, insErr := stmt.Exec(encoded, createdOn, modifiedOn, expiresOn)
	if insErr != nil {
		return insErr
	}
	lastInserted, lInsErr := res.LastInsertId()
	if lInsErr != nil {
		return lInsErr
	}
	session.ID = fmt.Sprintf("%d", lastInserted)
	return nil
}

func (m *MySQLStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {

	// Set cookie to expire.
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))
	// Clear session values.
	for k := range session.Values {
		delete(session.Values, k)
	}
	db, err := sql.Open("mysql", m.mysqlAuth)
	if err != nil {
		return err
	}
	defer db.Close()

	delQ := "DELETE FROM " + m.table + " WHERE id = ?"
	stmt, stmtErr := db.Prepare(delQ)
	if stmtErr != nil {
		return stmtErr
	}
	defer stmt.Close()
	_, delErr := stmt.Exec(session.ID)
	if delErr != nil {
		return delErr
	}
	return nil
}

func (m *MySQLStore) save(session *sessions.Session) error {
	if session.IsNew == true {
		return m.insert(session)
	}
	var createdOn time.Time
	var expiresOn time.Time
	crOn := session.Values["created_on"]
	if crOn == nil {
		createdOn = time.Now()
	} else {
		createdOn = crOn.(time.Time)
	}

	exOn := session.Values["expires_on"]
	if exOn == nil {
		expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
	} else {
		expiresOn = exOn.(time.Time)
		if expiresOn.Sub(time.Now().Add(time.Duration(session.Options.MaxAge)*time.Second)) < 0 {
			expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
		}
	}
	db, err := sql.Open("mysql", m.mysqlAuth)
	if err != nil {
		return err
	}
	defer db.Close()
	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")
	updQ := "UPDATE " + m.table + " SET session_data = ?, created_on = ?, expires_on = ? WHERE id = ?"
	stmt, stmtErr := db.Prepare(updQ)
	if stmtErr != nil {
		return stmtErr
	}
	defer stmt.Close()
	encoded, encErr := securecookie.EncodeMulti(session.Name(), session.Values, m.Codecs...)
	if encErr != nil {
		return encErr
	}
	_, updErr := stmt.Exec(encoded, createdOn, expiresOn, session.ID)
	if updErr != nil {
		return updErr
	}
	return nil
}

func (m *MySQLStore) load(session *sessions.Session) error {
	db, err := sql.Open("mysql", m.mysqlAuth)
	if err != nil {
		return err
	}
	defer db.Close()

	selQ := "SELECT id, session_data, created_on, modified_on, expires_on from " + m.table + " WHERE id = ?"
	stmt, stmtErr := db.Prepare(selQ)
	if stmtErr != nil {
		return stmtErr
	}
	defer stmt.Close()
	row := stmt.QueryRow(session.ID)
	sess := sessionRow{}
	scanErr := row.Scan(&sess.id, &sess.data, &sess.createdOn, &sess.modifiedOn, &sess.expiresOn)
	if sess.expiresOn.Sub(time.Now()) < 0 {
		return errors.New("Session expired")
	}
	if scanErr != nil {
		return scanErr
	}
	if err = securecookie.DecodeMulti(session.Name(), sess.data, &session.Values, m.Codecs...); err != nil {
		return err
	}
	session.Values["created_on"] = sess.createdOn
	session.Values["modified_on"] = sess.modifiedOn
	session.Values["expires_on"] = sess.expiresOn
	return nil

}
