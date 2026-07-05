#! /bin/bash
set -e
function kop() {
    git fetch
    # shellcheck disable=SC2086
    git tag -d ${version}
    # shellcheck disable=SC2086
    git push origin :refs/tags/${version}
    exit 0
}

git checkout main

git tag -l | tail -n 5
# shellcheck disable=SC2162
read -t 20 -p "[tag] delete version >>> " version
if [[ $version != "" ]]; then
  kop
  echo "[tag] Success deleted a tag: ${version}"
else
  echo  'No input delete version !'
fi

git checkout dev
echo '------------------------'