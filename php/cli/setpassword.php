<?php
// setpassword.php — set a known demo account's password, used at the end of
// a restore (backups never carry passwords; the console generates fresh ones
// and pushes them here).
//
//   php mdl-demo/cli/setpassword.php --username=jane --password=...
//
// A missing user is a warning, not an error: the account list in a backup is
// a snapshot, and one deleted inside Moodle before the backup must not abort
// the restore's final step.

define('CLI_SCRIPT', true);

require(__DIR__ . '/../init.php');
require_once($CFG->libdir . '/clilib.php');

list($options, $unrecognised) = cli_get_params([
    'username' => '',
    'password' => '',
], []);
if ($unrecognised) {
    cli_error('unrecognised options: ' . implode(', ', $unrecognised));
}
foreach (['username', 'password'] as $required) {
    if ($options[$required] === '') {
        cli_error("--$required is required");
    }
}

$user = core_user::get_user_by_username($options['username']);
if (!$user) {
    cli_writeln("warning: user '" . $options['username'] . "' not found, skipping");
    exit(0);
}
update_internal_user_password($user, $options['password']);
cli_writeln("password set for '" . $options['username'] . "'");
