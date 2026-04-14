package main

import (
	"fmt"

	slkdb "github.com/stevejackson/slk/internal/db"
)

func runSetStatus(args []string, status string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: slk %s <ts>", status)
	}
	ts := args[0]

	db, err := slkdb.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := db.Exec("UPDATE messages SET status=? WHERE ts=?", status, ts)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("message %s not found", ts)
	}
	fmt.Printf("%s → %s\n", ts, status)
	return nil
}
