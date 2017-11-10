package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	git "gopkg.in/src-d/go-git.v4"
)

const (
	AppName     = "gitfetch"
	HelpMessage = "Avilable commands:\n" +
		"\tadd PATH - add a repository to be fetched\n" +
		"\tremove PATH - remove a repository\n" +
		"\tlist - list repositories\n" +
		"\tworkers NUMBER - set a number of workers\n" +
		"\thelp - prints this help"
	SetWorkersMessage = "Specify repository path:\n" +
		"gitfetch workers NUMBER_OF_WORKERS"
	AddRepoMessage = "Specify a number of workers:\n" +
		"gitfetch add REPOSITORY_PATH"
	RemoveRepoMessage = "Specify repository path:\n" +
		"gitfetch remove REPOSITORY_PATH"
)

type Config struct {
	Workers      int      `json:"workers"`
	Repositories []string `json:"repositories"`
	configFile   string
	configPath   string
}

func openConfig() (*Config, error) {
	configPath := os.Getenv("XDG_CONFIG_HOME")

	if len(configPath) == 0 {
		configPath = path.Join(os.Getenv("HOME"), ".config")
	}
	configPath = path.Join(configPath, AppName)

	c := &Config{
		configPath: configPath,
		configFile: path.Join(configPath, AppName+".json"),
	}
	if err := c.read(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) read() error {
	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		c.Workers = 8
	}

	content, err := ioutil.ReadFile(c.configFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, c)
}

func (c *Config) AddRepo(repo string) {
	c.Repositories = append(c.Repositories, repo)
}

func (c *Config) DelRepo(repo string) error {
	toRemove := 0
	for ; toRemove < len(c.Repositories); toRemove++ {
		if c.Repositories[toRemove] == repo {
			break
		}
	}
	if toRemove < len(c.Repositories) {
		c.Repositories = append(
			c.Repositories[0:toRemove],
			c.Repositories[toRemove+1:]...)
		return nil
	}
	fmt.Println(repo, "not found")
	return fmt.Errorf("no such repo %s", repo)
}

func (c *Config) Close() error {
	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		if err = os.MkdirAll(c.configPath, 0700); err != nil {
			return err
		}
	}
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}
	ioutil.WriteFile(c.configFile, content, 0777)

	return nil
}

func (c *Config) FetchAll(ctx context.Context) <-chan bool {
	done := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(len(c.Repositories))
	for _, repopath := range c.Repositories {
		go func(repopath string) {
			defer wg.Done()
			fmt.Println("Fetching...", repopath)

			repo, err := git.PlainOpen(repopath)
			if err != nil {
				c.DelRepo(repopath)
				return
			}

			if err := repo.FetchContext(ctx, &git.FetchOptions{
				Progress: os.Stdout,
				Tags:     git.AllTags,
			}); err != nil {
				if err == git.NoErrAlreadyUpToDate {
					fmt.Println("already up to date")
				} else {
					fmt.Println("unexpected error", err.Error())
				}
			}
		}(repopath)
	}
	go func() {
		wg.Wait()
		done <- true
	}()
	return done
}

func main() {
	config, err := openConfig()
	if err != nil {
		panic(err)
	}
	defer config.Close()
	if len(os.Args) <= 1 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigs
			fmt.Println("singal received", sig, "closing")
			cancel()
		}()

		<-config.FetchAll(ctx)
		return
	}
	switch os.Args[1] {
	case "add":
		if len(os.Args) < 3 {
			fmt.Println(AddRepoMessage)
			return
		}
		if _, err := git.PlainOpen(os.Args[2]); err != nil {
			if err == git.ErrRepositoryNotExists {
				fmt.Println(os.Args[2], "is not a valid repository")
			} else {
				panic(err)
			}
		} else {
			config.AddRepo(os.Args[2])
		}
	case "workers":
		if len(os.Args) < 3 {
			fmt.Println(SetWorkersMessage)
		}
		if workers, err := strconv.Atoi(os.Args[2]); err != nil {
			fmt.Println(SetWorkersMessage)
		} else {
			config.Workers = workers
		}
	case "list":
		if len(config.Repositories) == 0 {
			fmt.Println("No repositories")
		} else {
			fmt.Println(strings.Join(config.Repositories, "\n"))
		}
	case "remove":
		if len(os.Args) < 3 {
			fmt.Println(RemoveRepoMessage)
		}
		if config.DelRepo(os.Args[2]); err != nil {
			fmt.Println(err.Error())
		}
	default:
		fmt.Println(HelpMessage)
	}
}
