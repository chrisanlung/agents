"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const ALL = "__all__"; // sentinel — Radix SelectItem cannot be empty string

interface PackageFilterProps {
  current: string;
}

export function PackageFilter({ current }: PackageFilterProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  function handleChange(value: string) {
    const params = new URLSearchParams(searchParams?.toString() ?? "");
    params.delete("page");
    if (value === ALL) {
      params.delete("package");
    } else {
      params.set("package", value);
    }
    const qs = params.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname);
  }

  return (
    <Select value={current || ALL} onValueChange={handleChange}>
      <SelectTrigger className="h-9 w-[160px] text-sm">
        <SelectValue placeholder="Semua paket" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>Semua paket</SelectItem>
        <SelectItem value="starter">Starter</SelectItem>
        <SelectItem value="growth">Growth</SelectItem>
        <SelectItem value="enterprise">Enterprise</SelectItem>
      </SelectContent>
    </Select>
  );
}
