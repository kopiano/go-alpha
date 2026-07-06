git checkout main
git merge dev
git pull origin main    # 已提交commit使用：git pull origin main --rebase
git push origin main
git checkout dev
echo '------------ merge success！------------'