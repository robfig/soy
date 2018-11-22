package soy

func Fuzz(data []byte) int {
	var _, err = NewBundle().
		AddGlobalsFile("testdata/FeaturesUsage_globals.txt").
		AddTemplateString("", string(data)).
		Compile()

	if err != nil {
		return 0
	}

	return 1
}
