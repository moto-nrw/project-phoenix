package application

import "testing"

// The Klassenfilter of an offering-sourced Regeltermin (#2482) moved with the
// roster resync from models/activities (#3560).
func TestSourceClassFilterMatches(t *testing.T) {
	t.Parallel()

	if !sourceClassFilterMatches(nil, "") || !sourceClassFilterMatches(nil, "3a") {
		t.Fatal("empty filter must admit every child")
	}
	if !sourceClassFilterMatches([]string{"1b"}, " 1B ") {
		t.Fatal("class matching must ignore case and padding")
	}
	if sourceClassFilterMatches([]string{"1b"}, "1a") {
		t.Fatal("class 1a must not match filter [1b]")
	}
	if sourceClassFilterMatches([]string{"1b"}, "") {
		t.Fatal("a child without a school class must not match a set filter")
	}
}

func TestGradeFilterMatches(t *testing.T) {
	t.Parallel()

	grade := int16(3)
	if !gradeFilterMatches(nil, nil) || !gradeFilterMatches(nil, &grade) {
		t.Fatal("empty filter must admit every child")
	}
	if !gradeFilterMatches([]int{2, 3}, &grade) {
		t.Fatal("grade 3 must match filter [2 3]")
	}
	if gradeFilterMatches([]int{2}, &grade) {
		t.Fatal("grade 3 must not match filter [2]")
	}
	if gradeFilterMatches([]int{3}, nil) {
		t.Fatal("a child without a derivable grade must not match a set filter")
	}
}
