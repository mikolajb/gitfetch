package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/libgit2/git2go"
)

const (
	AppName     = "gitfetch"
	HelpMessage = `Avilable commands:
\tadd PATH - add a repository to be fetched
\tremove PATH - remove a repository
\tlist - list repositories
\tworkers NUMBER - set a number of workers
\thelp - prints this help`
	SetWorkersMessage = `Specify repository path:
gitfetch workers NUMBER_OF_WORKERS`
	AddRepoMessage = `Specify repository path:
gitfetch add REPOSITORY_PATH`
	RemoveRepoMessage = `Specify repository path:
gitfetch remove REPOSITORY_PATH`
)

type Config struct {
	Workers      int      `json:"workers"`
	Repositories []string `json:"repositories"`
}

func getConfigFile() (string, string) {
	config_home := os.Getenv("XDG_CONFIG_HOME")

	if len(config_home) == 0 {
		config_home = path.Join(os.Getenv("HOME"), ".config")
	}

	config_home = path.Join(config_home, AppName)
	config_file := path.Join(config_home, AppName+".json")

	return config_file, config_home
}

func getConfig() *Config {
	config_file, _ := getConfigFile()

	if _, err := os.Stat(config_file); os.IsNotExist(err) {
		config := Config{
			Workers:      8,
			Repositories: []string{},
		}
		setConfig(&config)
	}

	config := new(Config)
	content, err := ioutil.ReadFile(config_file)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(content, config)

	return config
}

func setConfig(config *Config) {
	config_file, config_home := getConfigFile()
	if _, err := os.Stat(config_file); os.IsNotExist(err) {
		if err = os.MkdirAll(config_home, 0700); err != nil {
			panic(err)
		}
	}
	content, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(config_file, content, 0777)
}

func fetchRepo(repopaths chan string, control chan string) {
	for {
		repopath := <-repopaths
		if repopath == "" {
			break
		}
		repository, err := git.OpenRepository(repopath)
		if err != nil {
			fmt.Println(err)
		}
		iterator, err := repository.NewBranchIterator(git.BranchLocal)

		iterator.ForEach(func(
			branch *git.Branch,
			branch_type git.BranchType,
		) error {
			name, _ := branch.Name()
			ref, err := branch.Upstream()

			if err != nil {
				fmt.Println(repopath, err)

				return nil
			}

			remote_name, _ := repository.RemoteName(ref.Name())

			if ref.IsRemote() {
				remote, _ := repository.Remotes.Lookup(remote_name)

				fo := &git.FetchOptions{
					RemoteCallbacks: git.RemoteCallbacks{
						CertificateCheckCallback: func(
							cert *git.Certificate,
							valid bool,
							hostname string,
						) git.ErrorCode {

							if cert.Kind == git.CertificateX509 {
								err = cert.X509.VerifyHostname(hostname)
								if err != nil {
									fmt.Println(err)
									return git.ErrUser
								} else {
									return 0
								}
							} else if cert.Kind == git.CertificateHostkey {
								return git.ErrUser
							}

							return git.ErrUser
						},
						CredentialsCallback: func(
							url string,
							username_from_url string,
							allowed_types git.CredType,
						) (git.ErrorCode, *git.Cred) {
							if allowed_types&git.CredTypeUserpassPlaintext > 0 {
								return git.ErrUser, nil
							} else if allowed_types&git.CredTypeSshKey > 0 {
								ret, cred := git.NewCredSshKeyFromAgent(
									username_from_url,
								)
								return git.ErrorCode(ret), &cred
							} else if allowed_types&git.CredTypeSshCustom > 0 {
								return git.ErrUser, nil
							} else if allowed_types&git.CredTypeDefault > 0 {
								return git.ErrUser, nil
							}

							return git.ErrUser, nil
						},
					},
				}

				err = remote.Fetch([]string{name}, fo, "")

				if err != nil {
					fmt.Println("NOT FETCHED", err)
				}
			}
			return nil
		})
		control <- repopath
	}
}

func main() {
	config := getConfig()
	if len(os.Args) > 1 {
		if os.Args[1] == "add" {
			if len(os.Args) > 2 {
				_, err := git.OpenRepository(os.Args[2])
				if err != nil {
					fmt.Println(os.Args[2], "is not a valid repository")
				} else {
					config.Repositories = append(
						config.Repositories,
						os.Args[2],
					)
					setConfig(config)
				}
			} else {
				fmt.Println(AddRepoMessage)
			}
		} else if os.Args[1] == "workers" {
			if len(os.Args) > 2 {
				if workers, err := strconv.Atoi(os.Args[2]); err != nil {
					fmt.Println(SetWorkersMessage)
				} else {
					config.Workers = workers
				}
				setConfig(config)
			} else {
				fmt.Println(SetWorkersMessage)
			}
		} else if os.Args[1] == "list" {
			if len(config.Repositories) == 0 {
				fmt.Println("No repositories")
			} else {
				fmt.Println(strings.Join(config.Repositories, "\n"))
			}
		} else if os.Args[1] == "remove" {
			if len(os.Args) > 2 {
				toRemove := -1
				for i := 0; i < len(config.Repositories); i++ {
					if config.Repositories[i] == os.Args[2] {
						toRemove = i
						break
					}
				}
				if toRemove >= 0 {
					config.Repositories = append(
						config.Repositories[0:toRemove],
						config.Repositories[toRemove+1:]...)
					setConfig(config)
				} else {
					fmt.Println(os.Args[2], "not found")
				}
			} else {
				fmt.Println(RemoveRepoMessage)
			}
		} else {
			fmt.Println(HelpMessage)
		}
		return
	}

	r := make(chan string, config.Workers)
	c := make(chan string, config.Workers)

	for i := 0; i < config.Workers; i++ {
		go fetchRepo(r, c)
	}
	for _, repopath := range config.Repositories {
		fmt.Println("Fetching", repopath)
		r <- repopath
	}

	for i := 0; i < config.Workers; i++ {
		r <- ""
	}

	for range config.Repositories {
		fmt.Println("Done with...", <-c)
	}
}
