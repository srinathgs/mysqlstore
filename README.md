> This project is a modified fork of [srinathgs/mysqlstore](https://github.com/srinathgs/mysqlstore).

mysqlstore
==========

[![Build Status](https://travis-ci.org/zekroTJA/mysqlstore.svg?branch=master)](https://travis-ci.org/zekroTJA/mysqlstore)

Gorilla's Session Store Implementation for MySQL.

*Fixed version which is working with the current versions of Go.*

Installation
===========

Run `go get github.com/srinathgs/mysqlstore` from command line. Gets installed in `$GOPATH`.

Usage
=====

`NewMysqlStore` takes the following paramaters

```
endpoint - A sql.Open style endpoint
tableName - table where sessions are to be saved. Required fields are created automatically if the table doesnot exist.
path - path for Set-Cookie header
maxAge 
codecs
```

Internally, `mysqlstore` uses [this](https://github.com/go-sql-driver/mysql) MySQL driver.

Example:
```go
package main

import (
	"fmt"
	"github.com/srinathgs/mysqlstore"
	"net/http"
)

var store *mysqlstore.MySQLStore

func sessTest(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "foobar")
	session.Values["bar"] = "baz"
	session.Values["baz"] = "foo"
	err = session.Save(r, w)
	fmt.Printf("%#v\n", session)
	fmt.Println(err)
}

func main() {
	store, err := mysqlstore.NewMySQLStore("UN:PASS@tcp(<IP>:<PORT>)/<DB>?parseTime=true&loc=Local", <tablename>, "/", 3600, []byte("<SecretKey>"))
	if err != nil {
		panic(err)
	}
	defer store.Close()

	http.HandleFunc("/", sessTest)
	http.ListenAndServe(":8080", nil)
}
```
