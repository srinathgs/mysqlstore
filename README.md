mysqlstore
==========

Gorilla's Session Store Implementation for MySQL

Installation
===========

Run `go get github.com/srinathgs/mysqlstore` from command line. Gets installed in `$GOPATH`

Usage
=====

`NewMysqlStore` takes the following paramaters

    endpoint - A sql.Open style endpoint
    tableName - table where sessions are to be saved. Required fields are created automatically if the table doesnot exist.
    path - path for Set-Cookie header
    maxAge 
    codecs

e.g.,
      package main
  
      import (
  	    "fmt"
  	    "github.com/srinathgs/mysqlstore"
  	    "net/http"
      )
  
      var stor, _ = mysqlstore.NewMySQLStore("UN:PASS@tcp(<IP>:<PORT>)/<DB>?parseTime=true&loc=Local", <tablename>, "/", 3600, []byte("<SecretKey>"))
  
      func sessTest(w http.ResponseWriter, r *http.Request) {
  	    session, err := stor.Get(r, "foobar")
  	    session.Values["bar"] = "baz"
  	    session.Values["baz"] = "foo"
  	    err = session.Save(r, w)
  	    fmt.Printf("%#v\n", session)
  	    fmt.Println(err)
  	
      
      }

    func main() {
    	http.HandleFunc("/", sessTest)
    	http.ListenAndServe(":8080", nil)
    }
