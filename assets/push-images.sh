#!/bin/bash
images="vcluster-images.tar.gz"
list="vcluster-images.txt"
usage () {
    echo "USAGE: $0 [--images vcluster-images.tar.gz] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry:port."
    echo "  [-h|--help] Usage message"
}

push_manifest () {
    export DOCKER_CLI_EXPERIMENTAL=enabled
    manifest_list=()
    for i in "${arch_list[@]}"
    do
        manifest_list+=("$1-${i}")
    done

    echo "Preparing manifest $1, list[${arch_list[@]}]"
    docker manifest create "$1" "${manifest_list[@]}" --amend
    docker manifest push "$1" --purge
}

while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -r|--registry)
        reg="$2"
        shift # past argument
        shift # past value
        ;;
        -l|--image-list)
        list="$2"
        shift # past argument
        shift # past value
        ;;
        -i|--images)
        images="$2"
        shift # past argument
        shift # past value
        ;;
        -h|--help)
        help="true"
        shift
        ;;
        *)
        usage
        exit 1
        ;;
    esac
done
if [[ -z $reg ]]; then
    usage
    exit 1
fi
if [[ $help ]]; then
    usage
    exit 0
fi

docker load --input ${images}

linux_images=()
while IFS= read -r i; do
    [ -z "${i}" ] && continue
    linux_images+=("${i}");
done < "${list}"

arch_list+=("linux-amd64")
for i in "${linux_images[@]}"; do
    [ -z "${i}" ] && continue
    case $i in
    */*)
        image_name="${reg}/${i}"
        ;;
    *)
        image_name="${reg}/ghcr.io/loft-sh/${i}"
        ;;
    esac

    docker tag "${i}" "${image_name}"
    docker push "${image_name}"
done
