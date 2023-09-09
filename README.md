---
title: server docs
slug: docs
draft: false
---

# go-markdown-pages
articles, blog or portfolio. Powered by markdown


## development
```bash
# execute from repo root directory

# air
air -c .air.toml

#tailwindcss
./tw -w >public/tailwind.css
```

## TODOs

- [ ] have articles listed under /articles
- [ ] integrate with [turso.tech](https://turso.tech) (for fun and profit)
- [ ] animate page transitions
- [ ] syntax highlighting w/ https://github.com/wooorm/starry-night
- [ ] add search
- [ ] add tags
- [ ] add date
- [ ] add comments?
- [x] github flavored markdown and styling [css](https://github.com/sindresorhus/github-markdown-css)
- [x] enable html in markdown
- [x] add tailwindcss
- [x] add air
- [x] add markdown
- [x] update from github repo (requires git on server and a instatiated repo)

## build
```bash
# once per machine
cd ..
git clone https://github.com/johan-st/obsidian-vault

# sparse checkout will only pull the go-md-articles folder (optional)
cd obsidian-vault/
git sparse-checkout set go-md-articles

# once per build
git pull