package sqle

import (
	"context"
	"errors"
	"fmt"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/row"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/schema"
	"github.com/liquidata-inc/ld/dolt/go/store/types"
	"github.com/src-d/go-mysql-server/sql"
	"io"
)

// IndexDriver implementation. Not ready for prime time.

type DoltIndexDriver struct {
	db *Database
}

func (*DoltIndexDriver) ID() string {
	return "doltDbIndexDriver"
}

func (*DoltIndexDriver) Create(db, table, id string, expressions []sql.Expression, config map[string]string) (sql.Index, error) {
	panic("creating indexes not supported")
}

func (i *DoltIndexDriver) Save(*sql.Context, sql.Index, sql.PartitionIndexKeyValueIter) error {
	panic("saving indexes not supported")
}

func (i *DoltIndexDriver) Delete(sql.Index, sql.PartitionIter) error {
	panic("deleting indexes not supported")
}

func (i *DoltIndexDriver) LoadAll(db, table string) ([]sql.Index, error) {
	if db != i.db.name {
		panic("Unexpected db: " + db)
	}

	tbl, ok := i.db.root.GetTable(context.TODO(), table)
	if !ok {
		panic(fmt.Sprintf("No table found with name %s", table))
	}

	sch := tbl.GetSchema(context.TODO())
	return []sql.Index{ &doltIndex{sch, table, i.db, i} }, nil
}

type doltIndex struct {
	sch schema.Schema
	tableName string
	db *Database
	driver *DoltIndexDriver
}

func (di *doltIndex) Get(key ...interface{}) (sql.IndexLookup, error) {
	taggedVals, err := keyColsToTuple(di.sch, key)
	if err != nil {
		return nil, err
	}

	return &doltIndexLookup{di, taggedVals}, nil
}

func keyColsToTuple(sch schema.Schema, key []interface{}) (row.TaggedValues, error) {
	if sch.GetPKCols().Size() != len(key) {
		return nil, errors.New("key must specify all columns")
	}

	var i int
	taggedVals := make(row.TaggedValues)
	sch.GetPKCols().Iter(func(tag uint64, col schema.Column) (stop bool) {
		taggedVals[tag] = keyColToValue(key[i], col)
		i++
		return false
	})

	return taggedVals, nil
}

func keyColToValue(v interface{}, column schema.Column) types.Value {
	// TODO: type conversion
	switch column.Kind {
	case types.BoolKind:
		return types.Bool(v.(bool))
	case types.IntKind:
		return types.Int(v.(int64))
	case types.FloatKind:
		return types.Float(v.(float64))
	case types.UintKind:
		return types.Uint(v.(uint64))
	case types.UUIDKind:
		panic("Implement me")
	case types.StringKind:
		return types.String(v.(string))
	default:
		panic("unhandled type")
	}
}

func (*doltIndex) Has(partition sql.Partition, key ...interface{}) (bool, error) {
	// appears to be unused for the moment
	panic("implement me")
}

func (di *doltIndex) ID() string {
	return fmt.Sprintf("%s:primaryKey", di.tableName)
}

func (di *doltIndex) Database() string {
	return di.db.name
}

func (di *doltIndex) Table() string {
	return di.tableName
}

func (di *doltIndex) Expressions() []string {
	return primaryKeytoIndexStrings(di.tableName, di.sch)
}

// Returns the expression strings needed for this index to work. This needs to match the implementation in the sql
// engine, which requires $table.$column
func primaryKeytoIndexStrings(tableName string, sch schema.Schema) []string {
	colNames := make([]string, sch.GetPKCols().Size())
	var i int
	sch.GetPKCols().Iter(func(tag uint64, col schema.Column) (stop bool) {
		colNames[i] = tableName + "." + col.Name
		i++
		return true
	})
	return colNames
}

func (di *doltIndex) Driver() string {
	return di.driver.ID()
}

type doltIndexLookup struct {
	idx *doltIndex
	key row.TaggedValues
}

func (il *doltIndexLookup) Indexes() []string {
	return []string{il.idx.ID()}
}

// No idea what this is used for, examples aren't useful. From stepping through the code I know that we get index values
// by wrapping tables via the WithIndexLookup method. The iterator that this method returns yields []byte instead of
// sql.Row and its purpose is yet unclear.
func (il *doltIndexLookup) Values(p sql.Partition) (sql.IndexValueIter, error) {
	panic("implement me")
}

// RowIter returns a row iterator for this index lookup. The iterator will return the single matching row for the index.
func (il *doltIndexLookup) RowIter(ctx *sql.Context) (sql.RowIter, error) {
	return &indexLookupRowIterAdapter{indexLookup: il, ctx: ctx}, nil
}

type indexLookupRowIterAdapter struct {
	indexLookup *doltIndexLookup
	ctx *sql.Context
	i int
}

func (i *indexLookupRowIterAdapter) Next() (sql.Row, error) {
	if i.i > 0 {
		return nil, io.EOF
	}

	i.i++
	table, _ := i.indexLookup.idx.db.root.GetTable(i.ctx.Context, i.indexLookup.idx.tableName)
	r, ok := table.GetRowByPKVals(i.ctx.Context, i.indexLookup.key, i.indexLookup.idx.sch)
	if !ok {
		return nil, io.EOF
	}

	return doltRowToSqlRow(r, i.indexLookup.idx.sch), nil
}

func (*indexLookupRowIterAdapter) Close() error {
	return nil
}