git checkout dev
git add .
git status
read -t 40 -p "[dev] Enter commit >>> " message
if [ "$message" != "" ]; then
  git commit -m "[dev comment]: $message"
  git pull origin dev
  git push origin dev
  exit 0
else
  git reset
  echo "[dev] You haven't entered any comments !"
  exit 1
fi
git checkout dev

echo '------------------------'