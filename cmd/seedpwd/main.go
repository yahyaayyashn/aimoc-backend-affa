package main

import (
	"fmt"
	"log"
	"os"

	"aimoc-backend/internal/config"
	"aimoc-backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	db, err := repository.NewDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	pwd := "Admin@123"
	if len(os.Args) > 1 {
		pwd = os.Args[1]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	res := db.Exec("UPDATE users SET password_hash = ?", string(hash))
	if res.Error != nil {
		log.Fatal(res.Error)
	}
	fmt.Printf("OK: %d user password di-update ke '%s'\n", res.RowsAffected, pwd)
}
