# Run cast against an alternate Makefile via -f/--file.
Output demo/gifs/custom-makefile.gif
Set Width 1180
Set Height 230
Source demo/tapes/_setup.tape
Set TypingSpeed 40ms

Type "cast config | grep source" Sleep 400ms Enter
Sleep 1800ms
Type "cast -f Makefile.personal config | grep source" Sleep 400ms Enter
Sleep 2600ms
