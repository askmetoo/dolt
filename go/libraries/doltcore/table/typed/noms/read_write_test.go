package noms

import (
	"github.com/attic-labs/noms/go/spec"
	"github.com/attic-labs/noms/go/types"
	"github.com/google/uuid"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/row"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/schema"
	"github.com/liquidata-inc/ld/dolt/go/libraries/doltcore/table"
	"testing"
)

const (
	idCol       = "id"
	nameCol     = "name"
	ageCol      = "age"
	titleCol    = "title"
	idColTag    = 4
	nameColTag  = 3
	ageColTag   = 2
	titleColTag = 1
)

var colColl, _ = schema.NewColCollection(
	schema.NewColumn(idCol, idColTag, types.UUIDKind, true, schema.NotNullConstraint{}),
	schema.NewColumn(nameCol, nameColTag, types.StringKind, false),
	schema.NewColumn(ageCol, ageColTag, types.UintKind, false),
	schema.NewColumn(titleCol, titleColTag, types.StringKind, false),
)
var sch = schema.SchemaFromCols(colColl)

var uuids = []uuid.UUID{
	uuid.Must(uuid.Parse("00000000-0000-0000-0000-000000000000")),
	uuid.Must(uuid.Parse("00000000-0000-0000-0000-000000000001")),
	uuid.Must(uuid.Parse("00000000-0000-0000-0000-000000000002"))}
var names = []string{"Bill Billerson", "John Johnson", "Rob Robertson"}
var ages = []uint{32, 25, 21}
var titles = []string{"Senior Dufus", "Dufus", ""}

var updatedIndices = []bool{false, true, true}
var updatedAges = []uint{0, 26, 20}

func createRows(onlyUpdated, updatedAge bool) []row.Row {
	rows := make([]row.Row, 0, len(names))
	for i := 0; i < len(names); i++ {
		if !onlyUpdated || updatedIndices[i] {
			age := ages[i]
			if updatedAge && updatedIndices[i] {
				age = updatedAges[i]
			}

			rowVals := row.TaggedValues{
				idColTag:    types.UUID(uuids[i]),
				nameColTag:  types.String(names[i]),
				ageColTag:   types.Uint(age),
				titleColTag: types.String(titles[i]),
			}
			rows = append(rows, row.New(sch, rowVals))
		}
	}

	return rows
}

func TestReadWrite(t *testing.T) {
	dbSPec, _ := spec.ForDatabase("mem")
	db := dbSPec.GetDatabase()

	rows := createRows(false, false)

	initialMapVal := testNomsMapCreator(t, db, rows)
	testReadAndCompare(t, initialMapVal, rows)

	updatedRows := createRows(true, true)
	updatedMap := testNomsMapUpdate(t, db, initialMapVal, updatedRows)

	expectedRows := createRows(false, true)
	testReadAndCompare(t, updatedMap, expectedRows)
}

func testNomsMapCreator(t *testing.T, vrw types.ValueReadWriter, rows []row.Row) *types.Map {
	mc := NewNomsMapCreator(vrw, sch)
	return testNomsWriteCloser(t, mc, rows)
}

func testNomsMapUpdate(t *testing.T, vrw types.ValueReadWriter, initialMapVal *types.Map, rows []row.Row) *types.Map {
	mu := NewNomsMapUpdater(vrw, *initialMapVal, sch)
	return testNomsWriteCloser(t, mu, rows)
}

func testNomsWriteCloser(t *testing.T, nwc NomsMapWriteCloser, rows []row.Row) *types.Map {
	for _, r := range rows {
		err := nwc.WriteRow(r)

		if err != nil {
			t.Error("Failed to write row.", err)
		}
	}

	err := nwc.Close()

	if err != nil {
		t.Fatal("Failed to close writer")
	}

	err = nwc.Close()

	if err == nil {
		t.Error("Should give error for having already been closed")
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Should panic when writing a row after closing.")
			}
		}()

		nwc.WriteRow(rows[0])
	}()

	mapVal := nwc.GetMap()

	if mapVal == nil {
		t.Fatal("Map should not be nil")
	}

	return mapVal
}

func testReadAndCompare(t *testing.T, initialMapVal *types.Map, expectedRows []row.Row) {
	mr := NewNomsMapReader(*initialMapVal, sch)
	actualRows, numBad, err := table.ReadAllRows(mr, true)

	if err != nil {
		t.Fatal("Failed to read rows from map.")
	}

	if numBad != 0 {
		t.Error("Unexpectedly bad rows")
	}

	if len(actualRows) != len(expectedRows) {
		t.Fatal("Number of rows read does not match expectation")
	}

	for i := 0; i < len(expectedRows); i++ {
		if !row.AreEqual(actualRows[i], expectedRows[i], sch) {
			t.Error(row.Fmt(actualRows[i], sch), "!=", row.Fmt(expectedRows[i], sch))
		}
	}
}