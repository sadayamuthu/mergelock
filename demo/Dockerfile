# Records demo/demo.gif with VHS (https://github.com/charmbracelet/vhs).
# From the repository root:
#   gh release download -p 'mergelock_*_linux_arm64.tar.gz' -O - | tar -xz -C demo mergelock   # or linux_amd64
#   docker build -t mergelock-vhs demo
#   docker run --rm -v "$PWD/demo":/work -w /work mergelock-vhs demo.tape
FROM ghcr.io/charmbracelet/vhs
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY mergelock /usr/local/bin/mergelock
