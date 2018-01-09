# Trust Translator

Trust translator is a small utility to translate old keys, certificate chains and TRCs
to the new format.
The old format was used up until commit `072c31d2834083151d77f9c9523c3947599c08b3`.

Commits changing the format are:
- `ca4bce63fafd2d661c4564ca7ab7db8472077206` (certificate chain)
- `8096c31a0e94ef9a695ca419945f7d47cf754a88` (signing key)
- `64531d5da837be98d310846180f9e9cdc28f7ea0` (TRC)
- `41179839650e461d6f8e27af0e369b19114dd7e7` (online/offline key)

This utility helps with automation of translation from old to new format. It is 
structured in five sub tools:

1. __key_translator.py__: Translates old signing/online/offline keys to new format
2. __cert_translator.py__: Translates old certificate chains to new format
3. __trc_translator.py__: Translates old TRCs to new format
4. __sanity_check.py__: Checks that all certificate chains are still verifiable with 
the TRCs
5. __parse_tester.go__: Checks that all certificate chains and TRCs are parsable 
by go implementation

The translator scripts provide three different modes:
- __OUTDIR__: write new files to specified outdir
- __SUFFIX__: write new files to same dir with suffix added
- __OVERWRITE__: write new files to overwrite original files

Use -h for more information.

cert_translator.py and trc_translator.py both require that the conf dir already 
has the keys in the new format.

This is a sample snippet how to translate all keys, certificate chains and TRCs from
a gen folder in the old format: (WARNING: the original files are overwritten)
```
PYTHONPATH=.:python python3 tools/trust_translator/key_translator.py --mode overwrite gen/ISD*/AS*/b* gen/ISD*/AS*/cs* gen/ISD*/AS*/endhost gen/ISD*/AS*/ps* gen/ISD*/AS*/sb*
PYTHONPATH=.:python python3 tools/trust_translator/cert_translator.py -c gen/ISD*/AS*/cs* --mode overwrite gen/ISD*/AS*/*/certs/*.crt
PYTHONPATH=.:python python3 tools/trust_translator/trc_translator.py -c gen/ISD*/AS*/cs* --mode overwrite gen/ISD*/AS*/*/certs/*.trc
PYTHONPATH=.:python python3 tools/trust_translator/sanity_check.py -c gen/ISD*/AS*/*/certs/*.crt -t gen/ISD*/AS*/*/certs/*.trc
go run tools/trust_translator/parse_tester.go -t gen/ISD*/AS*/*/certs/*.trc -c gen/ISD*/AS*/*/certs/*.crt
```

<!---
PYTHONPATH=.:python python3 tools/trust_translator/key_translator.py --mode overwrite test-gen/ISD*/AS*/b* test-gen/ISD*/AS*/cs* test-gen/ISD*/AS*/endhost test-gen/ISD*/AS*/ps* test-gen/ISD*/AS*/sb*
PYTHONPATH=.:python python3 tools/trust_translator/cert_translator.py -c test-gen/ISD*/AS*/cs* --mode overwrite test-gen/ISD*/AS*/*/certs/*.crt
PYTHONPATH=.:python python3 tools/trust_translator/trc_translator.py -c test-gen/ISD*/AS*/cs* --mode overwrite test-gen/ISD*/AS*/*/certs/*.trc
PYTHONPATH=.:python python3 tools/trust_translator/sanity_check.py -c test-gen/ISD*/AS*/*/certs/*.crt -t test-gen/ISD*/AS*/*/certs/*.trc
go run tools/trust_translator/parse_tester.go -t test-gen/ISD*/AS*/*/certs/*.trc -c test-gen/ISD*/AS*/*/certs/*.crt
--->