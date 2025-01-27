# FRODO

(backronymed as Frequency Registry Of Dictionary Objects)


FRODO is a frequency database of corpora metadata and word forms. It is mostly used
along with CNC's other applications for fast overview data retrieval. In KonText, it's mainly
the "liveattrs" function, in WaG, it works as a core word/ngram dictionary.

For more information, see the [API.md](./API.md).

## API

see [API.md](./API.md) 🚧

## How to build the project

1. Get the sources (`git clone --depth 1 https://github.com/czcorpus/frodo.git`)
2. `go mod tidy`
3. `make`
