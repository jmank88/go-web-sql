package main

import (
	"database/sql"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-modules/modules"

	_ "github.com/lib/pq"
)

var (
	port           = flag.String("port", "8080", "Port to serve.")
	rowsLimit      = flag.Int("rowsLimit", 50, "Max number of rows to return.")
	driverName     = flag.String("driverName", "postgres", "Sql driver name.")
	dataSourceName = flag.String("dataSourceName", "postgres://postgres:postgres@localhost:5432?sslmode=disable", "Sql data source name.")
)

func main() {
	flag.Parse()

	templates, err := template.ParseFiles("web/index.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	config := &struct {
		Template  *template.Template `provide:""`
		RowsLimit int                `provide:"rowsLimit"`
	}{
		Template:  templates,
		RowsLimit: *rowsLimit,
	}

	pageServer := &indexServer{}

	dbModule := &dbModule{driverName: *driverName, dataSourceName: *dataSourceName}

	queryServer := &queryServer{}

	execServer := &execServer{}

	binder := modules.NewBinder(modules.Logger{os.Stdout})

	if err := binder.Bind(config, pageServer, queryServer, execServer, dbModule); err != nil {
		log.Fatal(err)
	}

	http.Handle("/index.html", pageServer)
	http.Handle("/query", queryServer)
	http.Handle("/execute", execServer)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatal(err)
	}
}

// A indexServer serves a blank landing page.
type indexServer struct {
	*template.Template `inject:""`
}

// Serves blank landing page.
func (s *indexServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.ExecuteTemplate(w, "index", nil); err != nil {
		http.Error(w, "failed to build page: "+err.Error(), http.StatusInternalServerError)
	}
}

// A queryServer handles sql queries.
type queryServer struct {
	*template.Template `inject:""`
	RowsLimit          int     `inject:"rowsLimit"`
	DB                 *sql.DB `inject:""`
}

// Performs query and serves a page with the results.
func (s *queryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "error parsing form: "+err.Error(), http.StatusBadRequest)
	}
	data := s.query(r.PostForm.Get("sql"))

	if err := s.ExecuteTemplate(w, "index", data); err != nil {
		http.Error(w, "failed to build page: "+err.Error(), http.StatusInternalServerError)
	}
}

// Queries query and returns pageData with results.
func (s *queryServer) query(query string) *PageData {
	log.Printf("querying: %s", query)
	rows, err := s.DB.Query(query)
	if err != nil {
		return &PageData{
			Query:   query,
			Results: QueryResults{Error: err},
		}
	}
	return &PageData{
		Query:   query,
		Results: *NewQueryResults(rows, s.RowsLimit),
	}
}

// An execServer handles sql execution.
type execServer struct {
	*template.Template `inject:""`
	DB                 *sql.DB `inject:""`
}

// Performs sql execution and serves page with results
func (s *execServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "error parsing form: "+err.Error(), http.StatusBadRequest)
	}
	data := s.execute(r.PostForm.Get("sql"))

	if err := s.ExecuteTemplate(w, "index", data); err != nil {
		http.Error(w, "failed to build page: "+err.Error(), http.StatusInternalServerError)
	}
}

// Executes query and returns pageData with results.
func (s *execServer) execute(query string) *PageData {
	log.Printf("executing: %s", query)
	//TODO use the result counts
	_, err := s.DB.Exec(query)
	return &PageData{
		Query:   query,
		Results: QueryResults{Error: err},
	}
}

// A dbModule provides a sql.DB instance.
type dbModule struct {
	driverName     string
	dataSourceName string

	DB *sql.DB `provide:""`
}

func (m *dbModule) Provide() error {
	if db, err := sql.Open(m.driverName, m.dataSourceName); err != nil {
		return err
	} else {
		m.DB = db
		return nil
	}
}

// Holds data for the webpage template.
type PageData struct {
	Query   string
	Results QueryResults
}

// Holds results from a query.
type QueryResults struct {
	Error   error
	Columns []string
	Data    [][]string
}

// Converts sql.Rows into QueryResults.
func NewQueryResults(rows *sql.Rows, rowLimit int) *QueryResults {
	columns, err := rows.Columns()
	if err != nil {
		return &QueryResults{Error: err}
	}
	data := make([][]string, 0)
	row := 1
	for rows.Next() && row < rowLimit {
		stringValues := make([]string, len(columns)+1)
		stringValues[0] = strconv.Itoa(row)
		pointers := make([]interface{}, len(columns))
		for i := 0; i < len(columns); i++ {
			pointers[i] = &stringValues[i+1]
		}
		if err := rows.Scan(pointers...); err != nil {
			return &QueryResults{Error: err}
		}
		data = append(data, stringValues)
		row += 1
	}
	return &QueryResults{
		Columns: append([]string{"Row"}, columns...),
		Data:    data,
	}
}
