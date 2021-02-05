package phpserialize

import (
	"log"
	"strings"
	"testing"
)

type token struct {
	AccessToken string `php:"access_token"`
	Expire      int    `php:"expires_in"`
	//ErrorCode    int    `php:"errcode"`
	//ErrorMessage string `php:"errmsg"`
	inKey string
}

type custom struct {
	data string
}

func (c custom) GetPHPClassName() string {
	return "test1"
}

func (c *custom) UnSerializePHP(data []byte) error {
	log.Println("Got data:", data)
	c.data = string(data)
	return nil
}

func TestDecoder_Decode(t *testing.T) {

	decoder := NewDecoder(strings.NewReader("a:2:{s:12:\"access_token\";s:131:\"NzMwMjAxYTkxYTc4YjY1OWIyYzAxOGUwZGRkNzA0YTctNjcxNmQxN2ZiODc4OGUxNTdhNTE1ZWI0NDc3MWVmNTgtNDQ2NjAyZDIzYmRiMDlkZTczNzZjMWM4MGExNzFhNzU\";s:10:\"expires_in\";i:1580725517;}"))
	//decoder := NewDecoder(strings.NewReader("a:3:{s:7:\"errcode\";i:1;s:6:\"errmsg\";s:30:\"\\xe7\\xb3\\xbb\\xe7\\xbb\\x9f\\xe5\\xbc\\x82\\xe5\\xb8\\xb8\\xef\\xbc\\x8c\\xe8\\xaf\\xb7\\xe7\\xa8\\x8d\\xe5\\x90\\x8e\\xe5\\x86\\x8d\\xe8\\xaf\\x95\";s:10:\"expires_in\";i:1612508763;}"))
	//decoder := NewDecoder(strings.NewReader("a:3:{s:7:\"errcode\";i:1;s:6:\"errmsg\";s:5:\"error\";s:10:\"expires_in\";i:1612508763;}"))
	//decoder := NewDecoder(strings.NewReader("s:6:\"你好\";"))
	//decoder := NewDecoder(strings.NewReader("d:1.7976931348623157E+308;"))
	//decoder := NewDecoder(strings.NewReader("a:4:{i:0;s:1:\"1\";i:1;s:1:\"2\";i:2;s:1:\"3\";i:3;s:1:\"4\";}"))
	//decoder := NewDecoder(strings.NewReader("b:1;"))

	var v token

	err := decoder.Decode(&v)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", v)

	var cu custom
	decoder = NewDecoder(strings.NewReader("C:5:\"test1\":3:{abc}"))
	err = decoder.Decode(&cu)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", cu)

}

func TestUnmarshal(t *testing.T) {

	var err error

	var tkp *token

	err = Unmarshal([]byte("N;"), &tkp)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tkp)

	var b bool
	err = Unmarshal([]byte("b:1;"), &b)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", b)

	err = Unmarshal([]byte("b:0;"), &b)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", b)

	var i int
	err = Unmarshal([]byte("i:1;"), &i)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", i)

	err = Unmarshal([]byte("i:0;"), &i)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", i)

	err = Unmarshal([]byte("i:-1;"), &i)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", i)

	err = Unmarshal([]byte("i:2147483647;"), &i)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", i)

	err = Unmarshal([]byte("i:-2147483647;"), &i)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", i)

	var f float64

	err = Unmarshal([]byte("d:1.123456789000000011213842299184761941432952880859375;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:1;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:0;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:-1;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:-1.123456789000000011213842299184761941432952880859375;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:100;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:51999999999999996980101120;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:8.529000000000000015048907821909675090775407218185526836754222629322086390857293736189603805541992188E-22;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	err = Unmarshal([]byte("d:8.9999999999999995265585574287341141808127531476202420890331268310546875E-9;"), &f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", f)

	var s string
	err = Unmarshal([]byte("s:5:\"hallo\";"), &s)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", s)

	var sb []byte
	err = Unmarshal([]byte("s:5:\"hallo\";"), &sb)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", s)

	var arr interface{}
	err = Unmarshal([]byte("a:6:{i:0;i:1;i:1;d:1.100000000000000088817841970012523233890533447265625;i:2;s:5:\"hallo\";i:3;N;i:4;b:1;i:5;a:0:{}}"), &arr)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", arr)

	var arrI []int

	err = Unmarshal([]byte("a:4:{i:0;i:1;i:1;i:2;i:2;i:3;i:3;i:4;}"), &arrI)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", arrI)

	var arrayAny []interface{}
	err = Unmarshal([]byte("a:6:{i:0;i:1;i:1;d:1.100000000000000088817841970012523233890533447265625;i:2;s:5:\"hallo\";i:3;N;i:4;b:1;i:5;a:1:{i:0;i:4;}}"), &arrayAny)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", arrayAny)

	var tkm map[string]string
	err = Unmarshal([]byte("a:2:{s:12:\"access_token\";s:131:\"NzMwMjAxYTkxYTc4YjY1OWIyYzAxOGUwZGRkNzA0YTctNjcxNmQxN2ZiODc4OGUxNTdhNTE1ZWI0NDc3MWVmNTgtNDQ2NjAyZDIzYmRiMDlkZTczNzZjMWM4MGExNzFhNzU\";s:10:\"expires_in\";i:1580725517;}"), &tkm)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tkm)

	var tk token
	err = Unmarshal([]byte("a:2:{s:12:\"access_token\";s:131:\"NzMwMjAxYTkxYTc4YjY1OWIyYzAxOGUwZGRkNzA0YTctNjcxNmQxN2ZiODc4OGUxNTdhNTE1ZWI0NDc3MWVmNTgtNDQ2NjAyZDIzYmRiMDlkZTczNzZjMWM4MGExNzFhNzU\";s:10:\"expires_in\";i:1580725517;}"), &tk)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tk)

	err = Unmarshal([]byte("O:5:\"token\":2:{s:19:\"\u0000token\u0000access_token\";s:131:\"NzMwMjAxYTkxYTc4YjY1OWIyYzAxOGUwZGRkNzA0YTctNjcxNmQxN2ZiODc4OGUxNTdhNTE1ZWI0NDc3MWVmNTgtNDQ2NjAyZDIzYmRiMDlkZTczNzZjMWM4MGExNzFhNzU\";s:17:\"\u0000token\u0000expires_in\";i:1580725517;}"), &tk)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tk)

	err = Unmarshal([]byte("O:5:\"token\":2:{s:19:\"\u0000token\u0000access_token\";s:131:\"NzMwMjAxYTkxYTc4YjY1OWIyYzAxOGUwZGRkNzA0YTctNjcxNmQxN2ZiODc4OGUxNTdhNTE1ZWI0NDc3MWVmNTgtNDQ2NjAyZDIzYmRiMDlkZTczNzZjMWM4MGExNzFhNzU\";s:17:\"\u0000token\u0000expires_in\";i:1580725517;}"), &tkm)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", tkm)

}
