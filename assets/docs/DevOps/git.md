# Git

## 常见需求

### 删除远程仓库的分支main
```sh
git push origin --delete dev
```
notic: `不能删除远程默认分支`（例如master），除非先去：github setting → default branch

### 修改远程分支名main
Git 不能直接 rename 远程分支，通常做法是：
1. 本地分支改名
2. 推送新分支到远程
3. 删除旧远程分支
```sh
git branch -m main dev
git checkout dev
git push origin dev
git push origin --delete main
```

### 修改远程仓库的默认分支main为master
Settings → Branches → Default branch → Change branch

### 删除分支dev
```sh
git branch -d dev
git push origin --delete dev
```