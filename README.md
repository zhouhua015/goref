goref
=====

A tool for golang, to find references of given identifier.

Dependencies
------------

You'll need rog-go repository to compile goref.

`go get -u code.google.com/p/rog-go/`

Installation 
------------

`go get -u github.com/zhouhua015/goref`

Usage
-----------

Well, it's exactly like `godef`, except without "-i" option.

`goref -f FILE_NAME -o 255 path/to/your/desired/directory`

Give `-f` and `-o` to specify file name and offset to find identifier, the last directory is the desired place wherever you want to search for references.

Note: The result will only reflect information from the _saved_ files. Save the changes if you want to get accurate result.

Editor Support
-------------

Currently, only a "lame" Vim plugin is available. :) 

TODO
--------------

 - **Support search of _promoted_ fields/methods.**
 - Recognize type switch identifier across multiple 'case's