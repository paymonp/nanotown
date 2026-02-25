package main

import (
	"fmt"
	"strconv"
)

func generateID(sm *SessionManager) string {
	max := 0
	for _, s := range sm.ListAll() {
		if n, err := strconv.Atoi(s.ID); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%d", max+1)
}
