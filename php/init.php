<?php
// init.php — shared Moodle bootstrap for the console's scripts in this
// directory tree: all version-specific magic lives here, the scripts just
// require this file (CLI scripts define CLI_SCRIPT first, as usual).
//
// config.php sits one level above the docroot's mdl-demo/ dir — or two, when
// the tree has the 5.1+ public/ split (config stays at the tree root).

require(file_exists(__DIR__ . '/../config.php')
    ? __DIR__ . '/../config.php' : __DIR__ . '/../../config.php');
