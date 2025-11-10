package bad_panic

func f() {
	panic("boom") // want "use of panic is forbidden"
}
