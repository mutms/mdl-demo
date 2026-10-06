<?php
// login.php — single-use login links for the mdl-demo console's "Log in…"
// buttons and QR codes. Tokens are minted by the console (internal/sso) into
// <dataroot>/mdldemo-sso/ and consumed here on first use.

require(__DIR__ . '/../config.php');

$token = optional_param('token', '', PARAM_ALPHANUMEXT);
$dir = $CFG->dataroot . '/mdldemo-sso';
$fail = function() {
    http_response_code(403);
    header('Content-Type: text/plain; charset=utf-8');
    die('This login link is invalid, expired, or was already used.');
};
if ($token === '' || !is_dir($dir)) {
    $fail();
}
$file = $dir . '/' . hash('sha256', $token) . '.json';
// Single use: claim the file with an atomic rename before reading it, so
// two requests with one token cannot both get in.
$claimed = $file . '.' . bin2hex(random_bytes(8)) . '.used';
if (!@rename($file, $claimed)) {
    $fail();
}
$data = @file_get_contents($claimed);
@unlink($claimed);
if ($data === false) {
    $fail();
}
$info = json_decode($data);
if (!$info || empty($info->username) || empty($info->expires) || $info->expires < time()) {
    $fail();
}
$user = get_complete_user_data('username', $info->username, null, false);
if (!$user || !empty($user->deleted) || !empty($user->suspended)) {
    $fail();
}
complete_user_login($user);
redirect($CFG->wwwroot . '/');
