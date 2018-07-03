# clamdscan notifier

This is a very simple way to hook up `clamdscan` to watch one or more
directories for new files, and then run the scanner on them. It will use
`zenity` to report scanning outcome in a dialog.

## Usage

```
sudo apt-get install zenity clamdscan
./clamnot ${HOME}/Downloads […] &
```

Simple debug output is written to stdout/stderr; better logging (including a
full copy of each scan result) is written to the journal.
