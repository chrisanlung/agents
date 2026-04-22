// Package main is a small operator utility: it takes a plaintext password on
// stdin (or via the -password flag) and emits an Argon2id hash plus a ready-
// to-paste UPDATE SQL for the bootstrap super-admin user.
//
// Usage examples:
//
//	# interactive (recommended — password does not appear in shell history)
//	./hashpw
//
//	# via env var
//	SUPER_ADMIN_INITIAL_PASSWORD='ChangeMeNow' ./hashpw
//
//	# via flag (visible in process list — dev only)
//	./hashpw -password 'ChangeMeNow'
//
// The emitted SQL targets the bootstrap user ID defined in ADR 0003
// (a0000000-0000-0000-0000-000000000001). Run it against the lustia database
// after golang-migrate has applied migrations 1..N.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chrisanlung/lustia-auth/internal/helper"
)

const bootstrapSuperAdminID = "a0000000-0000-0000-0000-000000000001"

func main() {
	var pw string
	flag.StringVar(&pw, "password", "", "plaintext password (dev only — prefer stdin or env)")
	flag.Parse()

	if pw == "" {
		pw = os.Getenv("SUPER_ADMIN_INITIAL_PASSWORD")
	}
	if pw == "" {
		fmt.Fprint(os.Stderr, "Enter password (will not be echoed by most terminals): ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "read password: %v\n", err)
			os.Exit(1)
		}
		pw = strings.TrimRight(line, "\r\n")
	}
	if pw == "" {
		fmt.Fprintln(os.Stderr, "error: password must not be empty")
		os.Exit(1)
	}
	if len(pw) < 12 {
		fmt.Fprintln(os.Stderr, "warning: password is shorter than 12 characters — lustia auth-service will enforce length on change")
	}

	hasher := helper.NewArgon2idHasher()
	hash, err := hasher.Hash(nil, pw) //nolint:staticcheck // argon2id does not use ctx
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash password: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("-- Copy the statement below and run it against the lustia database.")
	fmt.Println("-- must_change_password stays true (set by migration 000006) — the bootstrap")
	fmt.Println("-- user will be forced to rotate this password on first login.")
	fmt.Printf(
		"UPDATE \"user\"\n   SET password_hash = '%s'\n WHERE id = '%s';\n",
		hash, bootstrapSuperAdminID,
	)
}
