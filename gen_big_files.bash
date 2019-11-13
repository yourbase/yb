#!/bin/bash

ONE_MB=$((1024*1024))
ONE_GB=$(($ONE_MB*1024))

MB_FILES=${1:-6}
GB_FILES=${2:-6}

FILE_PREFIX="YBrnd_"

function gen_files() {
    base=${1:-based-somethin}
    max_size=${2:-100}
    n_files=${3:-9}
    unit="${4:-M}"

    min_size=25
    step=5

    # Safe limits?
    [ $unit = "G" -a $max_size -gt 2 ] && max_size=2

    if [ $max_size -lt 25 ]; then
        min_size=1
        step=1
    fi

    sz=$min_size
    for (( n=1; n<=$n_files; n++ )); do
        file_name="${base}_${sz}${unit}B.data"
        echo "#${n}: $file_name"

        [ $unit = "M" ] && multi=$ONE_MB
        [ $unit = "G" ] && multi=$ONE_GB

        tail -c $(( $n * $multi )) /dev/urandom > $file_name

        [ $sz -le $max_size ] && sz=$((sz + $step))
    done
}

function get_uuid() {
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w ${1:-16} | head -n 1
}

function rand_num() {
    cat /dev/urandom | tr -dc '1-9' | fold -w 256 | head -n 1 | head --bytes 1
}

function uniq_fname() {
    echo "${FILE_PREFIX}$(get_uuid)"
}

function generate_random_datafiles() {
    number_of_dirs=$(rand_num)
    depth_of_dirs=$(rand_num)
    number_of_files=$(rand_num)

    [ $number_of_dirs -gt 3 ] && number_of_dirs=3
    [ $number_of_files -gt 3 ] && number_of_files=3

    mkdir ./testdata
    pushd ./testdata
    i=1
    while [ $i -le $number_of_dirs ]; do
        dir_s="./"
        x=1
        while [ $x -le $depth_of_dirs ]; do
            dir_s="${dir_s}$(get_uuid)/"

            x=$(( $x + 1 ))
        done
        mkdir -p ${dir_s}
        echo "----- Generation $i of $number_of_dirs: 2 big ones and $number_of_files medium ones -----"
        # Create data files in leaf dirs
        gen_files "${dir_s}$(uniq_fname)" 40 $number_of_files "M"
        gen_files "${dir_s}$(uniq_fname)" 2 2 "G"

        i=$(( $i + 1 ))
    done
    popd

    gen_files "$(uniq_fname)" 35 2 "M"

}

generate_random_datafiles
