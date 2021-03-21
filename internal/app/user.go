package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/spf13/cobra"

	"codeberg.org/readeck/readeck/internal/auth/users"
)

func init() {
	rootCmd.AddCommand(addUserCmd)
}

var addUserCmd = &cobra.Command{
	Use:  "adduser",
	RunE: addUser,
}

func addUser(_ *cobra.Command, _ []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	password := string(bytePassword)

	return users.Users.Create(&users.User{
		Username: strings.TrimSpace(username),
		Password: password,
		Email:    email,
	})
}
