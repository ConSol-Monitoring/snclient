package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUtilsExpandDuration(t *testing.T) {
	tests := []struct {
		in  string
		res float64
	}{
		{"2d", 86400 * 2},
		{"1m", 60},
		{"10s", 10},
		{"100ms", 0.1},
		{"100", 100},
		{"-1h", -3600},
		{"100.00", 100},
	}

	for _, tst := range tests {
		res, err := ExpandDuration(tst.in)
		require.NoError(t, err)
		assert.InDeltaf(t, tst.res, res, 0.00001, "ExpandDuration: %s", tst.in)
	}
}

func TestUtilsIsFloatVal(t *testing.T) {
	tests := []struct {
		in  float64
		res bool
	}{
		{1.00, false},
		{1.01, true},
		{5, false},
	}

	for _, tst := range tests {
		res := IsFloatVal(tst.in)
		assert.Equalf(t, tst.res, res, "IsFloatVal: %s", tst.in)
	}
}

func TestUtilsExecPath(t *testing.T) {
	execPath, _, _, err := GetExecutablePath()
	require.NoErrorf(t, err, "GetExecutablePath works")

	assert.NotEmptyf(t, execPath, "GetExecutablePath")
}

func TestToPrecision(t *testing.T) {
	tests := []struct {
		in        float64
		precision int
		res       float64
	}{
		{1.001, 0, 1},
		{1.001, 3, 1.001},
		{1.0013, 3, 1.001},
	}

	for _, tst := range tests {
		res := ToPrecision(tst.in, tst.precision)
		assert.InDeltaf(t, tst.res, res, 0.00001, "ToPrecision: %v (precision: %d) -> %v", tst.in, tst.precision, res)
	}
}

func TestTokenizer(t *testing.T) {
	tests := []struct {
		in  string
		res []string
	}{
		{"", []string{""}},
		{"a bc d", []string{"a", "bc", "d"}},
		{"a 'bc' d", []string{"a", "'bc'", "d"}},
		{"a 'b c' d", []string{"a", "'b c'", "d"}},
		{`a "b'c" d`, []string{"a", `"b'c"`, "d"}},
		{`a 'b""c' d`, []string{"a", `'b""c'`, "d"}},
	}

	for _, tst := range tests {
		res := Tokenize(tst.in)
		assert.Equalf(t, tst.res, res, "Tokenize: %v -> %v", tst.in, res)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in  string
		res float64
	}{
		{"1.0", 1.0},
		{"0.1", 0.001},
		{"0.1.23", 0.001023},
	}

	for _, tst := range tests {
		res := ParseVersion(tst.in)
		assert.InDeltaf(t, tst.res, res, 0.00001, "ParseVersion: %v -> %v", tst.in, res)
	}
}

func TestDurationString(t *testing.T) {
	tests := []struct {
		in  time.Duration
		res string
	}{
		{time.Second * 5, "5000ms"},
		{time.Second * 90, "1m 30s"},
		{time.Minute * 5, "5m"},
		{time.Hour * 5, "05:00h"},
		{time.Hour * 24, "1d 00:00h"},
		{time.Hour * 200, "8d 08:00h"},
		{time.Hour * 800, "4w 5d"},
		{time.Hour * 12345, "1y 21w"},
		{time.Millisecond * -312, "-312ms"},
		{time.Nanosecond * -942, "-942ns"},
	}

	for _, tst := range tests {
		res := DurationString(tst.in)
		assert.Equalf(t, tst.res, res, "DurationString: %v -> %v", tst.in, res)
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		in  string
		res string
		err bool
	}{
		{`"test"`, `test`, false},
		{`'test'`, `test`, false},
		{`'test test'`, `test test`, false},
		{`"test test"`, `test test`, false},
		{`"test test`, "", true},
		{`'test test`, "", true},
		{`test"test`, `test"test`, false},
		{`test'test`, `test'test`, false},
		{`test test'`, "", true},
		{`test test"`, "", true},
		{`test='test'`, `test='test'`, false},
		{`test="test"`, `test="test"`, false},
	}

	for _, tst := range tests {
		res, err := TrimQuotes(tst.in)
		switch tst.err {
		case true:
			require.Errorf(t, err, "TrimQuotes should error on %s", tst.in)
		case false:
			require.NoErrorf(t, err, "TrimQuotes should not error on %s", tst.in)
			assert.Equalf(t, tst.res, res, "TrimQuotes: %v -> %v", tst.in, res)
		}
	}
}

func TestRankedSort(t *testing.T) {
	keys := []string{
		"/includes",
		"/settings/a",
		"/settings/b",
		"/settings/a/2",
		"/settings/b/1",
		"/settings/default",
		"/paths",
		"/modules",
	}
	expected := []string{
		"/paths",
		"/modules",
		"/settings/default",
		"/settings/a",
		"/settings/a/2",
		"/settings/b",
		"/settings/b/1",
		"/includes",
	}
	ranks := map[string]int{
		"/paths":            1,
		"/modules":          5,
		"/settings/default": 10,
		"/settings":         15,
		"default":           20,
		"/includes":         50,
	}

	sorted := SortRanked(keys, ranks)

	assert.Equalf(t, expected, sorted, "sorted by rank")
}

func TestSubtractSlice(t *testing.T) {

	numbersSlice1 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	numbersSlice2 := []int{1, 3, 5, 7, 9}

	numberSliceRes := SubtractSlice(numbersSlice1, numbersSlice2)
	numberSlicesExpected := []int{2, 4, 6, 8, 10}

	assert.ElementsMatch(t, numberSliceRes, numberSlicesExpected, "removing odd numbers from [1..10] should give even numbers.")
}

func TestMapsEqual(t *testing.T) {

	dataMaster := map[string]string{
		"x": "x",
		"y": "y",
		"z": "z",
	}

	copyUsingAssignment := dataMaster
	assert.True(t, MapsEqual(dataMaster, copyUsingAssignment), "map comparisons should be equal, assigned to new value")

	savedPtr := &dataMaster
	assert.True(t, MapsEqual(dataMaster, *savedPtr), "map comparisons should be equal, reference assigned to new value")

	type st struct {
		name    string
		data    map[string]string
		dataPtr *map[string]string
	}
	savedInStruct := st{
		name:    "anan",
		data:    dataMaster,
		dataPtr: &dataMaster,
	}
	assert.True(t, MapsEqual(dataMaster, savedInStruct.data), "map comparisons should be equal, assigned to new value in struct")
	assert.True(t, MapsEqual(dataMaster, *savedInStruct.dataPtr), "map comparisons should be equal, referece assigned to new value in struct")

	savedInList := []map[string]string{dataMaster}
	assert.True(t, MapsEqual(dataMaster, savedInList[0]), "map comparisons should be equal, saved to a list")

	savedInPtrList := [](*map[string]string){&dataMaster}
	assert.True(t, MapsEqual(dataMaster, *(savedInPtrList[0])), "map comparisons should be equal, referece saved to a list")

	// modifying master, the references should stay the same
	dataMaster["a"] = "a"
	dataMaster["b"] = "b"
	dataMaster["c"] = "c"
	dataMaster["d"] = "d"
	dataMaster["e"] = "e"
	dataMaster["f"] = "f"

	// check again after master is modified

	assert.True(t, MapsEqual(dataMaster, copyUsingAssignment), "map comparisons should be equal, assigned to new value")
	assert.Equal(t, "a", copyUsingAssignment["a"], "map variable should point to same table, assigned to new value")

	assert.True(t, MapsEqual(dataMaster, *savedPtr), "map comparisons should be equal, reference assigned to new value")
	assert.Equal(t, "b", (*savedPtr)["b"], "map variable should point to same table, reference assigned to new value")

	assert.True(t, MapsEqual(dataMaster, savedInStruct.data), "map comparisons should be equal, assigned to new value in struct")
	assert.Equal(t, "c", savedInStruct.data["c"], "map variable should point to same table, assigned to new value in struct")

	assert.True(t, MapsEqual(dataMaster, *savedInStruct.dataPtr), "map comparisons should be equal, referece assigned to new value in struct")
	assert.Equal(t, "d", savedInStruct.data["d"], "map variable should point to same table, referece assigned to new value in struct")

	assert.True(t, MapsEqual(dataMaster, savedInList[0]), "map comparisons should be equal, saved to a list")
	assert.Equal(t, "e", savedInStruct.data["e"], "map variable should point to same table, saved to a list")

	assert.True(t, MapsEqual(dataMaster, *(savedInPtrList[0])), "map comparisons should be equal, referece saved to a list")
	assert.Equal(t, "f", savedInStruct.data["f"], "map variable should point to same table, referece saved to a list")

	dataMaster2 := map[string]string{}
	assert.False(t, MapsEqual(dataMaster, dataMaster2), "map comparisons should be false, dataMaster2 is a new map")

	// fill up the newData to be same as dataMaster
	dataMaster2["a"] = "a"
	dataMaster2["b"] = "b"
	dataMaster2["c"] = "c"
	dataMaster2["d"] = "d"
	dataMaster2["e"] = "e"
	dataMaster2["f"] = "f"
	dataMaster2["x"] = "x"
	dataMaster2["y"] = "y"
	dataMaster2["z"] = "z"

	assert.Equal(t, dataMaster, dataMaster2, "both dataMaster and dataMaster2 has the same key-values, in the same order")
	assert.False(t, MapsEqual(dataMaster, dataMaster2), "map comparisons should be false, dataMaster2 has the same key-values but is a new table")

}

func TestContainsMap(t *testing.T) {

	map1 := map[string]string{
		"asd": "asd",
		"xyz": "xyz",
	}

	map1assigned := map1

	map2 := map[string]string{
		"snclient": "snclient",
	}

	map2referenceassined := &map2

	map3 := map[string]string{
		"foo": "foo",
		"bar": "bar",
	}

	map3inlist := []map[string]string{map3}

	list := []map[string]string{map1, map2, map3}

	assert.True(t, ContainsMap(list, map1), "should contain first map")
	assert.True(t, ContainsMap(list, map1assigned), "should contain first map assigned")

	assert.True(t, ContainsMap(list, map2), "should contain second map")
	assert.True(t, ContainsMap(list, *map2referenceassined), "should contain second map refernece assigned")

	assert.True(t, ContainsMap(list, map3), "should contain third map")
	assert.True(t, ContainsMap(list, map3inlist[0]), "should contain third map saved in list")

	map4 := map[string]string{
		"foo": " foo",
		"bar": "bar",
	}

	assert.False(t, ContainsMap(list, map4), "should not fourth map, it has same keys-values as map3 but is seperately initialized")
}
