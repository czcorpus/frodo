# FRODO

(backronymed as Frequency Registry Of Dictionary Objects)

FRODO is a frequency database for corpus metadata and word forms, exposed as an HTTP JSON API service.

## Key Features

* Imports NoSkE and SketchEngine vertical corpus files
* Uses MariaDB (MySQL) as the data backend
* Provides absolute, IPM, and ARF frequency information for each word form
* Supports two-level lemmatization ([see CNC Wiki](https://wiki.korpus.cz/doku.php/en:cnk:syn2020#lemmatization))
* Enables fast exploration of corpus structure (e.g., "Show all media types and authors when only fiction is selected")
* Searches for all (sub)lemmas containing a given word form, plus all their other forms
* Supports general n-grams, not limited to words

## Use Cases

At the CNC, FRODO is integrated with several applications:

* **KonText**
  * Query suggestions
  * Interactive subcorpus text type selection
* **Word at a Glance**
  * Fast word overview
  * Finding words with similar frequency (ARF)
  * Retrieving a lemma's word forms

For more information, see [API.md](./API.md).

## API

See [API.md](./API.md) 🚧

## Building the Project

1. Clone the repository: `git clone --depth 1 https://github.com/czcorpus/frodo.git`
2. Install dependencies: `go mod tidy`
3. Build: `make`
