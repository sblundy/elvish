use re

fn put-compile-s [name intype extraargs outtype]{
  extranames = (joins ', ' [(splits ', ' $extraargs | each [a]{ take 1 [(splits ' ' $a | all)]})])

  echo '
func (cp *compiler) '$name'Op(n '$intype$extraargs') '$outtype' {
	cp.compiling(n)
	return '$outtype'{cp.'$name'(n'$extranames'), n.Range().From, n.Range().To}
}

func (cp *compiler) '$name'Ops(ns []'$intype$extraargs') []'$outtype' {
	ops := make([]'$outtype', len(ns))
	for i, n := range ns {
		ops[i] = cp.'$name'Op(n'$extranames')
	}
	return ops
}'
}

fn generate-boilerplate [@files]{
  echo 'package eval

import "github.com/elves/elvish/parse"'

  cat $@files | each [line]{
    m = (re:find '^func \(cp \*compiler\) (\w+)\(\w+ ([^,\[\]]+)(.*)\) (\w*Op)Body \{$' $line)
    if (< 0 (count $m)) {
      put-compile-s $m[groups][1][text] $m[groups][2][text] $m[groups][3][text] $m[groups][4][text]
    }
  }
}

generate-boilerplate compile_effect.go compile_value.go > boilerplate.go
gofmt -w boilerplate.go