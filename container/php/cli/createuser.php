<?php
// createuser.php — create a demo account, run by the console (which owns the
// generated password and the SSO login links; creating users inside Moodle
// would leave the console blind to their credentials).
//
//   php mdl-demo/cli/createuser.php --username=jane --password=... \
//       --firstname=Jane --lastname=Doe [--role=manager|admin]
//
// --role=manager assigns the Manager role in the system context;
// --role=admin makes the account a site administrator.

define('CLI_SCRIPT', true);

require(__DIR__ . '/../../config.php');
require_once($CFG->libdir . '/clilib.php');
require_once($CFG->dirroot . '/user/lib.php');

list($options, $unrecognised) = cli_get_params([
    'username' => '',
    'password' => '',
    'firstname' => '',
    'lastname' => '',
    'role' => '',
], []);
if ($unrecognised) {
    cli_error('unrecognised options: ' . implode(', ', $unrecognised));
}
foreach (['username', 'password', 'firstname', 'lastname'] as $required) {
    if ($options[$required] === '') {
        cli_error("--$required is required");
    }
}
if (!in_array($options['role'], ['', 'manager', 'admin'], true)) {
    cli_error("--role must be 'manager', 'admin', or omitted");
}
if (core_user::get_user_by_username($options['username'])) {
    cli_error("user '" . $options['username'] . "' already exists");
}

$user = new stdClass();
$user->auth = 'manual';
$user->confirmed = 1;
$user->mnethostid = $CFG->mnet_localhost_id;
$user->username = $options['username'];
$user->password = $options['password'];
$user->firstname = $options['firstname'];
$user->lastname = $options['lastname'];
// noemailever is set, so the address only has to be unique and well-formed.
$user->email = $options['username'] . '@demo.invalid';
$userid = user_create_user($user, true, false);

if ($options['role'] === 'manager') {
    $roleid = $DB->get_field('role', 'id', ['shortname' => 'manager'], MUST_EXIST);
    role_assign($roleid, $userid, context_system::instance()->id);
} else if ($options['role'] === 'admin') {
    $admins = array_filter(explode(',', $CFG->siteadmins));
    $admins[] = $userid;
    set_config('siteadmins', implode(',', array_unique($admins)));
}

cli_writeln("created user '" . $options['username'] . "' (id $userid"
    . ($options['role'] !== '' ? ', role ' . $options['role'] : '') . ')');
