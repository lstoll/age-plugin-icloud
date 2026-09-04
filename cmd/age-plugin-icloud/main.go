package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"
	"lds.li/age-plugin-icloud/internal/identity"
)

func main() {
	generate := flag.Bool("generate", false, "generate a new identity in iCloud Keychain")
	list := flag.Bool("list", false, "reprint identity files from Keychain attributes")
	delete := flag.Bool("delete", false, "delete a named identity from iCloud Keychain")
	name := flag.String("name", identity.DefaultName, "identity name")
	accessControl := flag.String("access-control", string(identity.AccessControl5m), "`none`, `everyTime` (Touch ID each decrypt), or `5m` (Touch ID, skip 5 minutes on this Mac)")

	p, err := plugin.New(identity.PluginName)
	if err != nil {
		fatal(err)
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `age-plugin-icloud stores age MLKEM768-X25519 keys in iCloud Keychain.

Usage:
  age-plugin-icloud --generate [--name NAME] [--access-control=none|everyTime|5m]
  age-plugin-icloud --list [--name NAME]
  age-plugin-icloud --delete --name NAME

age invokes this plugin as age-plugin-icloud --age-plugin=identity-v1.

`)
		flag.PrintDefaults()
	}

	p.RegisterFlags(nil)
	flag.Parse()

	p.HandleIdentity(func(data []byte) (age.Identity, error) {
		return identity.LoadIdentity(string(data))
	})
	p.HandleIdentityAsRecipient(func(data []byte) (age.Recipient, error) {
		return identity.LoadRecipient(string(data))
	})

	n := 0
	if *generate {
		n++
	}
	if *list {
		n++
	}
	if *delete {
		n++
	}
	if n > 1 {
		fatal(fmt.Errorf("specify only one of --generate, --list, --delete"))
	}
	if flag.NArg() > 0 && n > 0 {
		fatal(fmt.Errorf("unexpected arguments: %v", flag.Args()))
	}

	nameSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "name" {
			nameSet = true
		}
	})

	switch {
	case *generate:
		mode, err := identity.ParseAccessControl(*accessControl)
		if err != nil {
			fatal(err)
		}
		file, err := identity.Generate(*name, mode)
		if err != nil {
			fatal(err)
		}
		fmt.Print(file)
	case *list:
		items, err := identity.List()
		if err != nil {
			fatal(err)
		}
		if nameSet {
			var found *identity.Item
			for i := range items {
				if items[i].Name == *name {
					found = &items[i]
					break
				}
			}
			if found == nil {
				fatal(fmt.Errorf("no identity named %q", *name))
			}
			items = []identity.Item{*found}
		}
		for _, item := range items {
			fmt.Print(identity.IdentityFile(item.Name, item.Recipient, time.Time{}, item.AccessControl))
		}
	case *delete:
		if !nameSet {
			fatal(fmt.Errorf("--delete requires --name"))
		}
		if err := identity.Delete(*name); err != nil {
			fatal(err)
		}
	default:
		sm := flag.Lookup("age-plugin")
		if sm != nil && sm.Value.String() != "" {
			os.Exit(p.Main())
		}
		flag.Usage()
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "age-plugin-icloud: %v\n", err)
	os.Exit(1)
}
