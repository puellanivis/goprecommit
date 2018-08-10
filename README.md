# goprecommit

You can setup git to automatically put your git-template into every git repo you initialize as well.

```
$ git config --global init.templatedir '~/.git_template'
```

Then, you can just put the files from the `git-template` directory from this repo into your `~/.git-template` directory.

You may need to run `git init` in any repos that you have already cloned though.
