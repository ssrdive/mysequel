// Package mysequel provides helpers to insert data directly to database
package mysequel

import (
	"database/sql"
	"errors"
	"reflect"

	sq "github.com/Masterminds/squirrel"
)

// InsertTable interface define table structure
// perform INSERT queries
type InsertTable interface {
	Name() string
	Cols() []string
	Values() []interface{}
	Transaction() *sql.Tx
}

// QueryRunner interface allows QueryToStructs to
// be passed in *sql.DB and *sql.Tx instances at the same time
type QueryRunner interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

// NewNullString fuctions returns a NULL if the passed string is empty
func NewNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func prepareInsert(tablename string, cols []string, values []interface{}) sq.InsertBuilder {
	return sq.Insert(tablename).Columns(cols...).Values(values...)
}

func executeInsert(tx *sql.Tx, ib sq.InsertBuilder) (int64, error) {
	result, err := ib.RunWith(tx).Exec()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	return id, nil
}

// Insert prepares INSERT statement and executes it
func Insert(t InsertTable) (int64, error) {
	ib := prepareInsert(t.Name(), t.Cols(), t.Values())
	id, err := executeInsert(t.Transaction(), ib)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UdpateTable interface define table structure
// perform UPDATE queries
type UdpateTable interface {
	InsertTable
	WhereCols() []string
	WhereValues() []interface{}
}

func prepareUpdate(tablename string, cols []string, values []interface{}, wcols []string, wvalues []interface{}) sq.UpdateBuilder {
	ub := sq.Update(tablename)
	for i, c := range cols {
		ub = ub.Set(c, values[i])
	}
	for i, c := range wcols {
		ub = ub.Where(sq.Eq{c: wvalues[i]})
	}
	return ub
}

func executeUpdate(tx *sql.Tx, ub sq.UpdateBuilder) (int64, error) {
	result, err := ub.RunWith(tx).Exec()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	r, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	return r, nil
}

// Update prepares UPDATE statement and executes it
func Update(t UpdateTable) (int64, error) {
	ub := prepareUpdate(t.Name(), t.Cols(), t.Values(), t.WhereCols(), t.WhereValues())
	r, err := executeUpdate(t.Transaction(), ub)
	if err != nil {
		return 0, err
	}
	return r, err
}

func rowsToStructs(rows *sql.Rows, dest interface{}) error {
	// Dereference the pointer to the slice passed
	destv := reflect.ValueOf(dest).Elem()

	// Create a struct of passed slice pointer type
	rowp := reflect.New(destv.Type().Elem())
	rowv := rowp.Elem()

	// Checks whether the struct field count matches the
	// length of the returned row columns
	if cols, _ := rows.Columns(); rowv.NumField() != len(cols) {
		return errors.New("Struct field count does not match column count")
	}

	args := make([]interface{}, rowv.NumField())
	for i := 0; i < rowv.NumField(); i++ {
		args[i] = rowv.Field(i).Addr().Interface()
	}

	// Loop through result set
	for rows.Next() {
		if err := rows.Scan(args...); err != nil {
			return err
		}

		destv.Set(reflect.Append(destv, rowv))
	}

	return nil
}

// QueryToStructs takes struct slice pointer, database instance, SQL query
// and placeholders and returns populates the slice with the result structs.
func QueryToStructs(dest interface{}, db QueryRunner, q string, args ...interface{}) error {
	rows, err := db.Query(q, args...)
	if err != nil {
		return err
	}

	defer rows.Close()
	return rowsToStructs(rows, dest)
}
