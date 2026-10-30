<?php
// Runner behind mdl-php: load the site's Moodle as a CLI script,
// then run a snippet or a file in the global scope, where $CFG, $DB and
// $USER are at hand.
//
// Arguments: <site tree> <username or ""> <"code"|"file"> <code or path> [args...]
// The snippet or file sees its own arguments in $argv, name first.

define('CLI_SCRIPT', true);

$mdlcli = (object)[
    'projectdir' => $argv[1] ?? '',
    'username' => $argv[2] ?? '',
    'mode' => $argv[3] ?? '',
    'source' => $argv[4] ?? '',
];
if (!is_file($mdlcli->projectdir . '/config.php')) {
    fwrite(STDERR, "mdl-php: config.php not found in '{$mdlcli->projectdir}'\n");
    exit(1);
}

$argv = array_merge(
    [$mdlcli->mode === 'file' ? $mdlcli->source : 'mdl-php'],
    array_slice($argv, 5)
);
$argc = count($argv);
$_SERVER['argv'] = $argv;
$_SERVER['argc'] = $argc;

require $mdlcli->projectdir . '/config.php';

if ($mdlcli->username !== '') {
    $mdlcli->user = $DB->get_record('user', ['username' => $mdlcli->username, 'deleted' => 0]);
    if (!$mdlcli->user) {
        fwrite(STDERR, "mdl-php: user '{$mdlcli->username}' not found\n");
        exit(1);
    }
    \core\session\manager::set_user($mdlcli->user);
}

if ($mdlcli->mode === 'file') {
    require $mdlcli->source;
} else {
    eval($mdlcli->source . ';');
}
