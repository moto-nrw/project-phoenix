package cmd

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestSeedDataGeneration(t *testing.T) {
	// Test array bounds issues
	roomIDs := make([]int64, 10)
	for i := 0; i < 10; i++ {
		roomIDs[i] = int64(i + 1)
	}

	// Test group assignment
	fmt.Println("Testing room assignment to groups:")
	for i := 0; i < 25; i++ {
		if i < 10 {
			fmt.Printf("Group %d gets room ID %d\n", i, roomIDs[i])
		} else {
			fmt.Printf("Group %d gets no room (nil)\n", i)
		}
	}

	// Test random seed
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("\nTesting random generation:")
	for i := 0; i < 5; i++ {
		fmt.Printf("Random: %f\n", rng.Float32())
	}

	// Test grade assignment
	grades := []string{"1A", "1B", "2A", "2B", "3A", "3B", "4A", "4B", "5A", "5B"}
	fmt.Println("\nTesting grade assignment for 120 students:")
	for i := 0; i < 120; i++ {
		gradeIndex := i % len(grades)
		if i < 10 || i >= 110 {
			fmt.Printf("Student %d -> Grade %s (index %d)\n", i, grades[gradeIndex], gradeIndex)
		}
	}
}
