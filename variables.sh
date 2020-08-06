# This should be included in both scripts, not invoked directly

if [ -z "${VERSION:-}" ]; then
  echo "Extracting version from tag ref ${YB_GIT_BRANCH:-}" 1>&2
  VERSION="$(echo "${YB_GIT_BRANCH:-}" | sed -e 's|refs/tags/||g')"
fi

if [ -z "$VERSION" ]; then
  echo "No version provided, won't release" 1>&2
  exit 1
fi

if $(echo $VERSION | grep -vqo '^v'); then
    echo "Doesn't start with a \"v\" when it should, not releasing"
    exit 1
fi

if $(echo $VERSION | grep -qo '\-[a-z]\+[0-9]*$'); then
    # Release candidate release
    CHANNEL="preview"
else
    # Stable releases should not have a suffix
    CHANNEL="stable"
fi

# Commit info
COMMIT="${YB_GIT_COMMIT:-}"
if [ -z "${COMMIT}" ]; then
    # If git is installed
    if hash git; then
        COMMIT="$(git rev-parse HEAD)"
    fi
fi

BUILD_COMMIT_INFO=""
if [ -n "${COMMIT}" ]; then
    BUILD_COMMIT_INFO=" -X 'main.commitSHA=$COMMIT'"
fi
