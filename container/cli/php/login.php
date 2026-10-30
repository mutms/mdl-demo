<?php
// Runner behind mdl-login; started through run.php, so Moodle is loaded. Mints
// the same single-use token the console's "Log in…" buttons use (internal/sso):
// only its hash reaches the dataroot, and login.php in the docroot consumes it.
//
// Arguments: <username> <local path>

[$mdlcliusername, $mdlclipath] = [$argv[1], $argv[2]];

if (!$DB->record_exists('user', ['username' => $mdlcliusername, 'deleted' => 0, 'suspended' => 0])) {
    fwrite(STDERR, "mdl-login: user '{$mdlcliusername}' not found\n");
    exit(1);
}
if ($mdlclipath === '' || $mdlclipath[0] !== '/') {
    fwrite(STDERR, "mdl-login: --path must start with /\n");
    exit(1);
}

$mdlclidir = $CFG->dataroot . '/mdldemo-sso';
if (!is_dir($mdlclidir) && !mkdir($mdlclidir, 0700)) {
    fwrite(STDERR, "mdl-login: cannot create {$mdlclidir}\n");
    exit(1);
}
$mdlclitoken = rtrim(strtr(base64_encode(random_bytes(32)), '+/', '-_'), '=');
file_put_contents($mdlclidir . '/' . hash('sha256', $mdlclitoken) . '.json', json_encode([
    'username' => $mdlcliusername,
    'expires' => time() + 600,
    'path' => $mdlclipath,
]));
echo $CFG->wwwroot, '/mdl-demo/login.php?token=', $mdlclitoken, "\n";
