// Child-only isolation: https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL
export function gitTestEnvironment(base = process.env) {
  const env = Object.fromEntries(Object.entries(base).filter(([key]) => !key.startsWith('GIT_')));
  return { ...env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
}

export function initGitFixture(git) {
  git('init', '--quiet', '--template=');
  // https://git-scm.com/docs/git-config#Documentation/git-config.txt-corehooksPath
  for (const [key, value] of Object.entries({
    'core.hooksPath': '/dev/null',
    'user.name': 'Git fixture',
    'user.email': 'fixture@example.invalid',
    'commit.gpgsign': 'false',
    'gc.auto': '0',
    'maintenance.auto': 'false',
  })) git('config', '--local', key, value);
}
