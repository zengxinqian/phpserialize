package phpserialize

import "testing"

func TestValid(t *testing.T) {

	testData := []string{
		"N;",
		"b:1;",
		"b:0;",
		"i:1;",
		"i:0;",
		"i:-1;",
		"i:2147483647;",
		"i:-2147483647;",
		"d:1.123456789000000011213842299184761941432952880859375;",
		"d:1;",
		"d:0;",
		"d:-1;",
		"d:-1.123456789000000011213842299184761941432952880859375;",
		"d:100;",
		"d:51999999999999996980101120;",
		"d:8.529000000000000015048907821909675090775407218185526836754222629322086390857293736189603805541992188E-22;",
		"d:8.9999999999999995265585574287341141808127531476202420890331268310546875E-9;",
		"s:5:\"hallo\";",
		"a:4:{i:0;i:1;i:1;i:2;i:2;i:3;i:3;i:4;}",
		"a:4:{i:0;s:1:\"1\";i:1;s:1:\"2\";i:2;s:1:\"3\";i:3;s:1:\"4\";}",
		"a:2:{s:3:\"abc\";s:3:\"def\";s:3:\"ghi\";s:3:\"jkl\";}",
		"a:0:{}",
		"O:10:\"testObject\":1:{s:1:\"a\";s:5:\"hallo\";}",
		"O:16:\"ÜberKööliäå\":1:{s:20:\"EåäöÅÄÖüÜber\";s:12:\"åäöÅÄÖ\";}",
		"C:5:\"test1\":3:{abd}",
	}

	for index, data := range testData {

		if err := checkValid([]byte(data), &scanner{}); err != nil {
			t.Fatal(err)
		}
		t.Logf("Test %d: %s ok", index, data)

	}

}

func TestScanEnd(t *testing.T) {

	data := []byte("O:10:\"testObject\":1:{s:1:\"a\";s:5:\"hallo\";}")

	scan := &scanner{}
	scan.reset()

	for i := 0; i < len(data); i++ {

		state := scan.step(data[i])
		if state == scanEnd {
			t.Logf("end pos: %d", i)
			break
		}
		if state == scanError {
			t.Fatal(scan.err)
		}

	}

}
