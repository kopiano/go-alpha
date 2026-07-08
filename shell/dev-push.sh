git checkout dev
git status
read -t 100 -p "[dev] Enter commit >>> " message
if [ "$message" != "" ]; then
  git commit -m "$message"
  git pull origin dev
  git push origin dev
  exit 0
else
  git reset
  echo "[dev] You haven't entered any comments !"
  exit 1
fi
git checkout dev

echo '----------- push success ！-------------'