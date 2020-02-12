package phpserialize

import (
	"testing"
)

type testObject struct {
	A string `php:"a"`
}

func (t testObject) GetPHPClassName() string {
	return "testSerial\\testObject"
}

type testExotic struct {
	EåäöÅÄÖüÜber string `php:åäöÅÄÖüÜber`
}

func (t testExotic) GetPHPClassName() string {
	return "ÜberKööliäå"
}

func TestEncoder_Encode(t *testing.T) {

	type testEntry struct {
		Value  interface{}
		Result string
	}

	testEntries := []testEntry{
		{
			Value:  nil,
			Result: "N;",
		},
		{
			Value:  true,
			Result: "b:1;",
		},
		{
			Value:  false,
			Result: "b:0;",
		},
		{
			Value:  1,
			Result: "i:1;",
		},
		{
			Value:  0,
			Result: "i:0;",
		},
		{
			Value:  -1,
			Result: "i:-1;",
		},
		{
			Value:  2147483647,
			Result: "i:2147483647;",
		},
		{
			Value:  -2147483647,
			Result: "i:-2147483647;",
		},
		{
			Value:  1.123456789,
			Result: "d:1.123456789;",
		},
		{
			Value:  1.0,
			Result: "d:1;",
		},
		{
			Value:  0.0,
			Result: "d:0;",
		},
		{
			Value:  -1.0,
			Result: "d:-1;",
		},
		{
			Value:  -1.123456789,
			Result: "d:-1.123456789;",
		},
		{
			Value:  1e2,
			Result: "d:100;",
		},
		{
			Value:  5.2e25,
			Result: "d:5.2E+25;",
		},
		{
			Value:  85.29e-23,
			Result: "d:8.529E-22;",
		},
		{
			Value:  9e-9,
			Result: "d:9E-9;",
		},
		{
			Value:  "hallo",
			Result: "s:5:\"hallo\";",
		},
		{
			Value:  []interface{}{1, 1.1, "hallo", nil, true, []interface{}{}},
			Result: "a:6:{i:0;i:1;i:1;d:1.1;i:2;s:5:\"hallo\";i:3;N;i:4;b:1;i:5;a:0:{}}",
		},
		{
			Value:  testObject{A: "hallo"},
			Result: "O:21:\"testSerial\\testObject\":1:{s:1:\"a\";s:5:\"hallo\";}",
		},
		{
			Value:  &testObject{A: "hallo"},
			Result: "O:21:\"testSerial\\testObject\":1:{s:1:\"a\";s:5:\"hallo\";}",
		},
		{
			Value:  []interface{}{4},
			Result: "a:1:{i:0;i:4;}",
		},
		{
			Value:  []interface{}{4.5},
			Result: "a:1:{i:0;d:4.5;}",
		},
		{
			Value:  map[string]string{"åäöÅÄÖ": "åäöÅÄÖ"},
			Result: "a:1:{s:12:\"åäöÅÄÖ\";s:12:\"åäöÅÄÖ\";}",
		},
		{
			Value:  testExotic{EåäöÅÄÖüÜber: "åäöÅÄÖ"},
			Result: "O:16:\"ÜberKööliäå\":1:{s:20:\"EåäöÅÄÖüÜber\";s:12:\"åäöÅÄÖ\";}",
		},
	}

	for index, entry := range testEntries {

		result, err := Marshal(entry.Value)
		if err != nil {
			t.Fatal(err)
		}
		testResult := string(result)
		t.Logf("Test %d result: %s", index, testResult)

		if testResult != entry.Result {
			t.Fatalf("Test fail at index %d, expect:%s got %s", index, entry.Result, testResult)
		}

	}

	t.Log("all tests passed.")

}

func TestEncoder_Encode_Precision(t *testing.T) {

	SerializePrecision = 100

	type testEntry struct {
		Value  interface{}
		Result string
	}

	testEntries := []testEntry{
		{
			Value:  nil,
			Result: "N;",
		},
		{
			Value:  true,
			Result: "b:1;",
		},
		{
			Value:  false,
			Result: "b:0;",
		},
		{
			Value:  1,
			Result: "i:1;",
		},
		{
			Value:  0,
			Result: "i:0;",
		},
		{
			Value:  -1,
			Result: "i:-1;",
		},
		{
			Value:  2147483647,
			Result: "i:2147483647;",
		},
		{
			Value:  -2147483647,
			Result: "i:-2147483647;",
		},
		{
			Value:  1.123456789,
			Result: "d:1.123456789000000011213842299184761941432952880859375;",
		},
		{
			Value:  1.0,
			Result: "d:1;",
		},
		{
			Value:  0.0,
			Result: "d:0;",
		},
		{
			Value:  -1.0,
			Result: "d:-1;",
		},
		{
			Value:  -1.123456789,
			Result: "d:-1.123456789000000011213842299184761941432952880859375;",
		},
		{
			Value:  1e2,
			Result: "d:100;",
		},
		{
			Value:  5.2e25,
			Result: "d:51999999999999996980101120;",
		},
		{
			Value:  85.29e-23,
			Result: "d:8.529000000000000015048907821909675090775407218185526836754222629322086390857293736189603805541992188E-22;",
		},
		{
			Value:  9e-9,
			Result: "d:8.9999999999999995265585574287341141808127531476202420890331268310546875E-9;",
		},
		{
			Value:  "hallo",
			Result: "s:5:\"hallo\";",
		},
		{
			Value:  []interface{}{1, 1.1, "hallo", nil, true, []interface{}{}},
			Result: "a:6:{i:0;i:1;i:1;d:1.100000000000000088817841970012523233890533447265625;i:2;s:5:\"hallo\";i:3;N;i:4;b:1;i:5;a:0:{}}",
		},
		{
			Value:  testObject{A: "hallo"},
			Result: "O:21:\"testSerial\\testObject\":1:{s:1:\"a\";s:5:\"hallo\";}",
		},
		{
			Value:  &testObject{A: "hallo"},
			Result: "O:21:\"testSerial\\testObject\":1:{s:1:\"a\";s:5:\"hallo\";}",
		},
		{
			Value:  []interface{}{4},
			Result: "a:1:{i:0;i:4;}",
		},
		{
			Value:  []interface{}{4.5},
			Result: "a:1:{i:0;d:4.5;}",
		},
		{
			Value:  map[string]string{"åäöÅÄÖ": "åäöÅÄÖ"},
			Result: "a:1:{s:12:\"åäöÅÄÖ\";s:12:\"åäöÅÄÖ\";}",
		},
		{
			Value:  testExotic{EåäöÅÄÖüÜber: "åäöÅÄÖ"},
			Result: "O:16:\"ÜberKööliäå\":1:{s:20:\"EåäöÅÄÖüÜber\";s:12:\"åäöÅÄÖ\";}",
		},
	}

	for index, entry := range testEntries {

		result, err := Marshal(entry.Value)
		if err != nil {
			t.Fatal(err)
		}
		testResult := string(result)
		t.Logf("Test %d result: %s", index, testResult)

		if testResult != entry.Result {
			t.Fatalf("Test fail at index %d, expect:%s got %s", index, entry.Result, testResult)
		}

	}

	t.Log("all tests passed.")

}
