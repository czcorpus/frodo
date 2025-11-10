
#!/usr/bin/env python3
"""
Generate liveattrs processing of multiple vertical files from a single directory
Usage: python list_files.py <directory_path>
"""

import os
import sys
import json

LA_CONF = {
    "verticalFiles": [],
    "ngrams": {
        "ngramSize": 1,
        "calcARF": True,
        "vertColumns": [
            { "idx": 0, "role": "word" },
            { "idx": 2, "role": "lemma" },
            { "idx": 3, "role": "sublemma" },
            { "idx": 4, "role": "tag" }
        ]
    },
    "maxNumErrors": 10000000,
    "atomStructure": "text"
}

NGRAM_CONF = {
    'posTagset': 'cs_cnc2020',
    'colMapping': {'word': 0, 'lemma': 2, 'sublemma': 3, 'tag': 4}
}


COMMAND_TPL = 'curl -XPOST "http://10.0.3.33:8081/liveAttributes/online_2_wag/data?append=0&aliasOf=online_2&reconfigure=1" -H "Accept: application/json" -d \'{}\''
NGRAM_TPL = 'curl -XPOST "http://10.0.3.33:8081/dictionary/online_2_wag/ngrams?append={}&aliasOf=online_2" -H "Accept: application/json" -d \'{}\''

def list_files(directory):
    ans = []
    if not os.path.exists(directory):
        raise Exception(f"Error: Directory '{directory}' does not exist.", file=sys.stderr)

    if not os.path.isdir(directory):
        raise Exception(f"Error: '{directory}' is not a directory.", file=sys.stderr)

    try:
        for item in os.listdir(directory):
            full_path = os.path.join(directory, item)
            if os.path.isfile(full_path):
                exp_path = os.path.join('/var/opt/kontext/vertikaly/monitora', item)
                ans.append(exp_path)
    except PermissionError:
        raise Exception(f"Error: Permission denied to access '{directory}'.", file=sys.stderr)
    return ans


def main():
    if len(sys.argv) != 2:
        print("Usage: python list_files.py <directory_path>", file=sys.stderr)
        sys.exit(1)

    directory = sys.argv[1]
    LA_CONF['verticalFiles'] = list_files(directory)
    print(COMMAND_TPL.format(json.dumps(LA_CONF)))
    print(NGRAM_TPL.format(0, json.dumps(NGRAM_CONF)))


if __name__ == "__main__":
    main()
