<script lang="ts">
    import type {Item} from "$lib/gallery";
    import {PUBLIC_API_PATH} from '$env/static/public'

    interface Props {
        item: Item;
    }

    /*
    src="full.jpg"
      srcset="
        250p.webp 250w,
        800p.webp 800w,
        1920p.webp 1920w"
      sizes="
        (max-width: 600px) 250px,
        (max-width: 1200px) 800px,
        1920px"
      alt="Responsive image"
     */

    let {item}: Props = $props();

</script>

<!--     srcset={`-->
<!--        /assets/${item.guid}/${item.guid}-250p.webp 250w,-->
<!--        /assets/${item.guid}/${item.guid}-800p.webp 800w,-->
<!--        /assets/${item.guid}/${item.guid}-1920p.webp 1920w`}-->

<img src={`${PUBLIC_API_PATH}/assets/${item.guid}`}
     sizes="
        (max-width: 600px) 250px,
        (max-width: 1200px) 800px,
        (min-width: 1200px) 1200px,
        1920px"
     width={item.width} height={item.height}
     alt={item.guid}/>
