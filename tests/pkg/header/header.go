package bbb

type Header struct {
	Name  string
	Value int
}

type TestStruct struct {
	Name   string
	header *Header

	Result []string
}

type TestStructEntry struct {
	file  string
	tests []*TestStruct
}

var entries = []*TestStructEntry{
	{
		file: "test1.go",
		tests: []*TestStruct{
			{
				Name: "1111",
				header: &Header{
					Name:  "1111",
					Value: 22222,
				},
				Result: []string{"333", "4444", "5555"},
			},
			{
				Name: "1111",
				header: &Header{
					Name:  "6666",
					Value: 7777,
				},
				Result: []string{"888", "9999", "101010"},
			},
			{
				Name: "1111",
				header: &Header{
					Name:  "111111",
					Value: 121212,
				},
				Result: []string{"131313", "14141414", "151515"},
			},
		},
	},
	{
		file: "test1.go",
		tests: []*TestStruct{
			{
				Name: "1111",
				header: &Header{
					Name:  "1111",
					Value: 22222,
				},
				Result: []string{"333", "4444", "5555"},
			},
			{
				header: &Header{
					Name:  "6666",
					Value: 7777,
				},
				Result: []string{"888", "9999", "101010"},
			},
			{
				header: &Header{
					Name:  "111111",
					Value: 121212,
				},
				Result: []string{"131313", "14141414", "151515"},
			},
		},
	},
	{
		file: "test1.go",
		tests: []*TestStruct{
			{
				header: &Header{
					Name:  "1111",
					Value: 22222,
				},
				Result: []string{"333", "4444", "5555"},
			},
			{
				header: &Header{
					Name:  "6666",
					Value: 7777,
				},
				Result: []string{"888", "9999", "101010"},
			},
			{
				header: &Header{
					Name:  "111111",
					Value: 121212,
				},
				Result: []string{"131313", "14141414", "151515"},
			},
		},
	},
}
