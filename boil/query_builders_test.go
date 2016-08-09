package boil

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

var writeGoldenFiles = flag.Bool(
	"test.golden",
	false,
	"Write golden files.",
)

func TestBuildQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		q    *Query
		args []interface{}
	}{
		{&Query{from: []string{"t"}}, nil},
		{&Query{from: []string{"q"}, limit: 5, offset: 6}, nil},
		{&Query{from: []string{"q"}, orderBy: []string{"a ASC", "b DESC"}}, nil},
		{&Query{from: []string{"t"}, selectCols: []string{"count(*) as ab, thing as bd", `"stuff"`}}, nil},
		{&Query{from: []string{"a", "b"}, selectCols: []string{"count(*) as ab, thing as bd", `"stuff"`}}, nil},
		{&Query{
			selectCols: []string{"a.happy", "r.fun", "q"},
			from:       []string{"happiness as a"},
			joins:      []join{{clause: "rainbows r on a.id = r.happy_id"}},
		}, nil},
		{&Query{
			from:  []string{"happiness as a"},
			joins: []join{{clause: "rainbows r on a.id = r.happy_id"}},
		}, nil},
		{&Query{
			from:    []string{"a"},
			groupBy: []string{"id", "name"},
			having:  []string{"id <> 1", "length(name, 'utf8') > 5"},
		}, nil},
		{&Query{
			delete: true,
			from:   []string{"thing happy", `upset as "sad"`, "fun", "thing as stuff", `"angry" as mad`},
			where: []where{
				where{clause: "a=$1", args: []interface{}{}},
				where{clause: "b=$2", args: []interface{}{}},
				where{clause: "c=$3", args: []interface{}{}},
			},
		}, nil},
		{&Query{
			delete: true,
			from:   []string{"thing happy", `upset as "sad"`, "fun", "thing as stuff", `"angry" as mad`},
			where: []where{
				where{clause: "(id=$1 and $thing=$2) or stuff=$3", args: []interface{}{}},
			},
		}, nil},
	}

	for i, test := range tests {
		filename := filepath.Join("_fixtures", fmt.Sprintf("%02d.sql", i))
		out, args := buildQuery(test.q)

		if *writeGoldenFiles {
			err := ioutil.WriteFile(filename, []byte(out), 0664)
			if err != nil {
				t.Fatalf("Failed to write golden file %s: %s\n", filename, err)
			}
			t.Logf("wrote golden file: %s\n", filename)
			continue
		}

		byt, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Fatalf("Failed to read golden file %q: %v", filename, err)
		}

		if string(bytes.TrimSpace(byt)) != out {
			t.Errorf("[%02d] Test failed:\nWant:\n%s\nGot:\n%s", i, byt, out)
		}

		if !reflect.DeepEqual(args, test.args) {
			t.Errorf("[%02d] Test failed:\nWant:\n%s\nGot:\n%s", i, spew.Sdump(test.args), spew.Sdump(args))
		}
	}
}

func TestIdentifierMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		In  Query
		Out map[string]string
	}{
		{
			In:  Query{from: []string{`a`}},
			Out: map[string]string{"a": "a"},
		},
		{
			In:  Query{from: []string{`"a"`, `b`}},
			Out: map[string]string{"a": "a", "b": "b"},
		},
		{
			In:  Query{from: []string{`a as b`}},
			Out: map[string]string{"b": "a"},
		},
		{
			In:  Query{from: []string{`a AS "b"`, `"c" as d`}},
			Out: map[string]string{"b": "a", "d": "c"},
		},
		{
			In:  Query{joins: []join{{kind: JoinInner, clause: `a on stuff = there`}}},
			Out: map[string]string{"a": "a"},
		},
		{
			In:  Query{joins: []join{{kind: JoinNatural, clause: `"a" on stuff = there`}}},
			Out: map[string]string{"a": "a"},
		},
		{
			In:  Query{joins: []join{{kind: JoinNatural, clause: `a as b on stuff = there`}}},
			Out: map[string]string{"b": "a"},
		},
		{
			In:  Query{joins: []join{{kind: JoinOuterRight, clause: `"a" as "b" on stuff = there`}}},
			Out: map[string]string{"b": "a"},
		},
	}

	for i, test := range tests {
		m := identifierMapping(&test.In)

		for k, v := range test.Out {
			val, ok := m[k]
			if !ok {
				t.Errorf("%d) want: %s = %s, but was missing", i, k, v)
			}
			if val != v {
				t.Errorf("%d) want: %s = %s, got: %s", i, k, v, val)
			}
		}
	}
}

func TestWriteStars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		In  Query
		Out []string
	}{
		{
			In:  Query{from: []string{`a`}},
			Out: []string{`"a".*`},
		},
		{
			In:  Query{from: []string{`a as b`}},
			Out: []string{`"b".*`},
		},
		{
			In:  Query{from: []string{`a as b`, `c`}},
			Out: []string{`"b".*`, `"c".*`},
		},
		{
			In:  Query{from: []string{`a as b`, `c as d`}},
			Out: []string{`"b".*`, `"d".*`},
		},
	}

	for i, test := range tests {
		selects := writeStars(&test.In)
		if !reflect.DeepEqual(selects, test.Out) {
			t.Errorf("writeStar test fail %d\nwant: %v\ngot:  %v", i, test.Out, selects)
		}
	}
}

func TestWhereClause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		q      Query
		expect string
	}{
		// Where("a=$1")
		{
			q: Query{
				where: []where{
					where{clause: "a=$1"},
				},
			},
			expect: " WHERE a=$1",
		},
		// Where("a=$1 OR b=$2")
		{
			q: Query{
				where: []where{
					where{clause: "a=$1 OR b=$2"},
				},
			},
			expect: " WHERE a=$1 OR b=$2",
		},
		// Where("a=$1", "b=$2")
		{
			q: Query{
				where: []where{
					where{clause: "a=$1"},
					where{clause: "b=$2"},
				},
			},
			expect: " WHERE a=$1 AND b=$2",
		},
		// Where("(a=$1 AND b=$2) OR c=$3")
		{
			q: Query{
				where: []where{
					where{clause: "(a=$1 AND b=$2) OR c=$3"},
				},
			},
			expect: " WHERE (a=$1 AND b=$2) OR c=$3",
		},
		// Where("a=$1 OR b=$2", "c=$3 OR d=$4 OR e=$5")
		{
			q: Query{
				where: []where{
					where{clause: "(a=$1 OR b=$2)"},
					where{clause: "(c=$3 OR d=$4 OR e=$5)"},
				},
			},
			expect: " WHERE (a=$1 OR b=$2) AND (c=$3 OR d=$4 OR e=$5)",
		},
		// Where("(a=$1 AND b=$2) OR (c=$3 AND d=$4 AND e=$5) OR f=$6 OR f=$7")
		{
			q: Query{
				where: []where{
					where{clause: "(a=$1 AND b=$2) OR (c=$3 AND d=$4 AND e=$5) OR f=$6 OR g=$7"},
				},
			},
			expect: " WHERE (a=$1 AND b=$2) OR (c=$3 AND d=$4 AND e=$5) OR f=$6 OR g=$7",
		},
		// Where("(a=$1 AND b=$2) OR (c=$3 AND d=$4 OR e=$5) OR f=$6 OR g=$7")
		{
			q: Query{
				where: []where{
					where{clause: "(a=$1 AND b=$2) OR (c=$3 AND d=$4 OR e=$5) OR f=$6 OR g=$7"},
				},
			},
			expect: " WHERE (a=$1 AND b=$2) OR (c=$3 AND d=$4 OR e=$5) OR f=$6 OR g=$7",
		},
	}

	for i, test := range tests {
		result, _ := whereClause(&test.q)
		if result != test.expect {
			t.Errorf("%d) Mismatch between expect and result:\n%s\n%s\n", i, test.expect, result)
		}
	}
}

func TestWriteAsStatements(t *testing.T) {
	t.Parallel()

	query := Query{
		selectCols: []string{
			`a`,
			`a.fun`,
			`"b"."fun"`,
			`"b".fun`,
			`b."fun"`,
			`a.clown.run`,
			`COUNT(a)`,
		},
	}

	expect := []string{
		`"a"`,
		`"a"."fun" as "a.fun"`,
		`"b"."fun" as "b.fun"`,
		`"b"."fun" as "b.fun"`,
		`"b"."fun" as "b.fun"`,
		`"a"."clown"."run" as "a.clown.run"`,
		`COUNT(a)`,
	}

	gots := writeAsStatements(&query)

	for i, got := range gots {
		if expect[i] != got {
			t.Errorf(`%d) want: %s, got: %s`, i, expect[i], got)
		}
	}
}
