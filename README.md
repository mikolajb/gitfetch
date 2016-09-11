# Gitfetch

`gitfetch` command fetches remotes for all branches that has its upstream set

Repository includes systemd service and timer that allows to use gitfetch as a daemon

## How to use

You can use the following commands
- `gitfetch` - perform fetch for set repositories
- `gitfetch add PATH` - add a repository to be fetched
- `gitfetch remove PATH`  - remove a repository
- `gitfetch list` - list repositories
- `gitfetch workers NUMBER`  - set a number of workers
- `gitfetch help` - prints help

Gitfetch configuration is stored in `~/.config/gitfetch/`.

## How to install

- Install Go (if you don't have it)
- and compile `go build gitfetch.go`

## How to use as a daemon

- place files `gitfetch.timer` and `gitfetch.service` in `~/.config/systemd/user/`
- modify variable `ExecStart` in `gitfetch.service` - it should point to the executable of gitfetch.

Then, execute the following commands:

```
systemctl --user enable gitfetch.timer
systemctl --user start gitfetch.timer
```
